package ngstate

import (
	"encoding/hex"
	"math/big"
	"sync"

	"github.com/c0mm4nd/rlp"

	"github.com/dgraph-io/badger/v3"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
)

// var snapshot *atomic.Value

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

// GetSnapshot return the snapshot in a balance sheet at a height, and doo hash check
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

// GetSnapshotByHeight return the snapshot in a balance sheet at a height, without hash check
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

// GetSnapshotByHash return the snapshot in a balance sheet with the hash
// for internal use only
func (sm *SnapshotManager) GetSnapshotByHash(hash []byte) *ngtypes.Sheet {
	sm.RLock()
	defer sm.RLocker()

	return sm.hashToSnapshot[hex.EncodeToString(hash)]
}

// generateSnapshot when the block is a checkpoint
func (state *State) generateSnapshot(txn *badger.Txn) error {
	accounts := make([]*ngtypes.Account, 0)
	balances := make([]*ngtypes.Balance, 0)
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
		err = rlp.DecodeBytes(rawAccount, &account)
		if err != nil {
			return err
		}

		accounts = append(accounts, &account)
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

		balances = append(balances, &ngtypes.Balance{
			Address: addr,
			Amount:  new(big.Int).SetBytes(rawBalance),
		})
	}

	sheet := ngtypes.NewSheet(state.Network, latestBlock.Header.Height, latestBlock.GetHash(), balances, accounts)
	state.SnapshotManager.PutSnapshot(latestBlock.Header.Height, latestBlock.GetHash(), sheet)
	return nil
}

func (state *State) GetSnapshot(height uint64, hash []byte) *ngtypes.Sheet {
	return state.SnapshotManager.GetSnapshot(height, hash)
}
