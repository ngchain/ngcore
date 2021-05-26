package ngstate

import (
	"encoding/hex"
	"sync"

	"github.com/dgraph-io/badger/v3"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

//var snapshot *atomic.Value

type SnapshotManager struct {
	sync.RWMutex
	heightToHash   map[uint64]string
	hashToSnapshot map[string]*ngtypes.Sheet // hash->sheet
}

func (sm *SnapshotManager) PutSnapshot(height uint64, hash []byte, sheet *ngtypes.Sheet) {
	sm.Lock()
	defer sm.Unlock()

	hexHash := hex.EncodeToString(hash)

	sm.heightToHash[height] = hexHash
	sm.hashToSnapshot[hexHash] = sheet
}

// for external use with security ensure
func (sm *SnapshotManager) GetSnapshot(height uint64, hash []byte) *ngtypes.Sheet {
	sm.RLock()
	defer sm.RLocker()

	hexHash, exists := sm.heightToHash[height]
	if !exists {
		return nil
	}

	if hexHash != hex.EncodeToString(hash) {
		return nil
	}

	return sm.hashToSnapshot[hexHash]
}

// for internal use only
func (sm *SnapshotManager) GetSnapshotByHeight(height uint64) *ngtypes.Sheet {
	sm.RLock()
	defer sm.RLocker()

	hexHash, exists := sm.heightToHash[height]
	if !exists {
		return nil
	}

	return sm.hashToSnapshot[hexHash]
}

// for internal use only
func (sm *SnapshotManager) GetSnapshotByHash(hash []byte) *ngtypes.Sheet {
	sm.RLock()
	defer sm.RLocker()

	return sm.hashToSnapshot[hex.EncodeToString(hash)]
}

// generateSnapshot when the block is a checkpoint
func (state *State) generateSnapshot(txn *badger.Txn) error {
	accounts := make(map[uint64]*ngtypes.Account)
	anonymous := make(map[string][]byte)
	latestBlock, err := ngblocks.GetLatestBlock(txn)
	if err != nil {
		return err
	}

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()
	for it.Seek(numToAccountPrefix); it.ValidForPrefix(numToAccountPrefix); it.Next() {
		item := it.Item()
		rawAccount, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		var account ngtypes.Account
		err = utils.Proto.Unmarshal(rawAccount, &account)
		if err != nil {
			return err
		}

		accounts[account.Num] = &account
	}

	it = txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()
	for it.Seek(addrToBalancePrefix); it.ValidForPrefix(addrToBalancePrefix); it.Next() {
		item := it.Item()
		addr := item.KeyCopy(nil)
		rawBalance, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		anonymous[base58.FastBase58Encoding(addr)] = rawBalance
	}

	sheet := ngtypes.NewSheet(latestBlock.PrevBlockHash, accounts, anonymous)
	state.SnapshotManager.PutSnapshot(latestBlock.Height, latestBlock.Hash(), sheet)
	return nil
}

func (state *State) GetSnapshot(height uint64, hash []byte) *ngtypes.Sheet {
	return state.SnapshotManager.GetSnapshot(height, hash)
}
