package ngstate

import (
	"encoding/hex"
	"math/big"
	"sync"

	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// var snapshot *atomic.Value

type SnapshotManager struct {
	sync.RWMutex
	heightToHash   map[uint64]string
	hashToSnapshot map[string]*ngtypes.Sheet // hash->sheet
}

// snapshotRetention bounds how many checkpoint rounds of snapshots stay
// in memory (must cover the mature-balance lookback window)
const snapshotRetention = 16

func (sm *SnapshotManager) PutSnapshot(height uint64, hash []byte, sheet *ngtypes.Sheet) {
	sm.Lock()
	defer sm.Unlock()

	// prune everything older than the retention window
	if height > snapshotRetention*ngtypes.BlockCheckRound {
		floor := height - snapshotRetention*ngtypes.BlockCheckRound
		for h, hexHash := range sm.heightToHash {
			if h < floor {
				delete(sm.hashToSnapshot, hexHash)
				delete(sm.heightToHash, h)
			}
		}
	}

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

// GenerateSnapshotTxn captures the current state as the sheet of the
// latest block (a checkpoint) inside the given txn, making it servable
// to snapshot-syncing peers
func (state *State) GenerateSnapshotTxn(txn *bbolt.Tx) error {
	accounts := make([]*ngtypes.Account, 0)
	balances := make([]*ngtypes.Balance, 0)

	blockBucket := txn.Bucket(storage.BlockBucketName)
	latestBlock, err := ngblocks.GetLatestBlock(blockBucket)
	if err != nil {
		return err
	}

	num2accBucket := txn.Bucket(storage.Num2AccBucketName)
	c := num2accBucket.Cursor()
	for num, rawAccount := c.First(); num != nil; num, rawAccount = c.Next() {
		var account ngtypes.Account
		err = rlp.DecodeBytes(rawAccount, &account)
		if err != nil {
			return err
		}

		accounts = append(accounts, &account)
	}

	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)
	c = addr2balBucket.Cursor()

	for addr, rawBalance := c.First(); addr != nil; addr, rawBalance = c.Next() {
		balances = append(balances, &ngtypes.Balance{
			Address: new(ngtypes.Address).SetBytes(addr),
			Amount:  new(big.Int).SetBytes(rawBalance),
		})
	}

	sheet := ngtypes.NewSheet(state.Network, latestBlock.GetHeight(), latestBlock.GetHash(), balances, accounts)
	state.SnapshotManager.PutSnapshot(latestBlock.GetHeight(), latestBlock.GetHash(), sheet)
	return nil
}

func (state *State) GetSnapshot(height uint64, hash []byte) *ngtypes.Sheet {
	return state.SnapshotManager.GetSnapshot(height, hash)
}
