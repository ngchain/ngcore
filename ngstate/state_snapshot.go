package ngstate

import (
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"sync"

	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
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
	defer sm.RUnlock()

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
	defer sm.RUnlock()

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
	defer sm.RUnlock()

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

	contractBucket := txn.Bucket(storage.ContractBucketName)
	c := contractBucket.Cursor()
	for addr, rawAccount := c.First(); addr != nil; addr, rawAccount = c.Next() {
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

	return state.PutSnapshotTxn(txn, sheet)
}

// PutSnapshotTxn caches the sheet in memory AND persists it in the
// snapshot bucket (pruned by the retention window), so mature-balance
// lookups survive restarts
func (state *State) PutSnapshotTxn(txn *bbolt.Tx, sheet *ngtypes.Sheet) error {
	state.SnapshotManager.PutSnapshot(sheet.Height, sheet.BlockHash, sheet)

	snapshotBucket := txn.Bucket(storage.SnapshotBucketName)

	raw, err := rlp.EncodeToBytes(sheet)
	if err != nil {
		return err
	}
	if err := snapshotBucket.Put(utils.PackUint64LE(sheet.Height), raw); err != nil {
		return err
	}

	// prune persisted snapshots below the retention window
	if sheet.Height > snapshotRetention*ngtypes.BlockCheckRound {
		floor := sheet.Height - snapshotRetention*ngtypes.BlockCheckRound
		c := snapshotBucket.Cursor()
		for k, _ := c.First(); k != nil && binary.LittleEndian.Uint64(k) < floor; k, _ = c.Next() {
			if err := c.Delete(); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetSnapshotByHeight resolves the state sheet at the height: the
// in-mem cache first, then the persisted bucket (exact height, else the
// nearest OLDER one as a conservative floor); height 0 is always the
// genesis sheet
func (state *State) GetSnapshotByHeight(height uint64) *ngtypes.Sheet {
	if height == 0 {
		return ngtypes.GetGenesisSheet(state.Network)
	}

	if sheet := state.SnapshotManager.GetSnapshotByHeight(height); sheet != nil {
		return sheet
	}

	var sheet *ngtypes.Sheet
	_ = state.View(func(txn *bbolt.Tx) error {
		snapshotBucket := txn.Bucket(storage.SnapshotBucketName)

		raw := snapshotBucket.Get(utils.PackUint64LE(height))
		if raw == nil {
			// nearest older snapshot: better a conservative floor than
			// an error (pre-persistence dbs, retention gaps)
			c := snapshotBucket.Cursor()
			var candidate []byte
			for k, v := c.First(); k != nil && binary.LittleEndian.Uint64(k) <= height; k, v = c.Next() {
				candidate = v
			}
			raw = candidate
		}
		if raw == nil {
			return nil
		}

		var s ngtypes.Sheet
		if err := rlp.DecodeBytes(raw, &s); err != nil {
			log.Errorf("broken persisted snapshot@%d: %v", height, err)
			return nil
		}
		sheet = &s

		return nil
	})

	if sheet == nil {
		return ngtypes.GetGenesisSheet(state.Network)
	}

	state.SnapshotManager.PutSnapshot(sheet.Height, sheet.BlockHash, sheet)

	return sheet
}

func (state *State) GetSnapshot(height uint64, hash []byte) *ngtypes.Sheet {
	return state.SnapshotManager.GetSnapshot(height, hash)
}
