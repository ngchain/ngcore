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
)

// snapshotKey encodes a snapshot's height big-endian, so the persisted keys
// sort in numeric height order (the retention prune and the nearest-older
// lookup both walk the bucket in order)
func snapshotKey(height uint64) []byte {
	var k [8]byte
	binary.BigEndian.PutUint64(k[:], height)
	return k[:]
}

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

// DumpSheetTxn captures the current state as a self-contained sheet of
// the latest block: every balance, every contract (code + context) and
// the key registry. This is the full-state export snapshot sync AND
// rpc-based forking (`getSheet` / `ngcore fork --rpc`) are built on.
func DumpSheetTxn(network ngtypes.Network, txn *bbolt.Tx) (*ngtypes.Sheet, error) {
	latestBlock, err := ngblocks.GetLatestBlock(txn.Bucket(storage.BlockBucketName))
	if err != nil {
		return nil, err
	}
	return DumpSheetAt(network, txn, latestBlock.GetHeight(), latestBlock.GetHash())
}

// DumpSheetAt dumps the state buckets of the txn into a sheet tagged with
// the given height/hash — for whole-state reconstructions whose db has no
// block store (see ReconstructAt)
func DumpSheetAt(network ngtypes.Network, txn *bbolt.Tx, height uint64, blockHash []byte) (*ngtypes.Sheet, error) {
	contracts := make([]*ngtypes.Contract, 0)
	balances := make([]*ngtypes.Balance, 0)

	c := txn.Bucket(storage.ContractBucketName).Cursor()
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		var addr ngtypes.Address
		copy(addr[:], k)
		// resolve the code so the sheet carries a self-contained
		// snapshot (setContract re-dedups it on apply)
		account, err := getContract(txn, addr)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, account)
	}

	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)
	c = addr2balBucket.Cursor()

	for addr, rawBalance := c.First(); addr != nil; addr, rawBalance = c.Next() {
		balances = append(balances, &ngtypes.Balance{
			Address: new(ngtypes.Address).SetBytes(addr),
			Amount:  new(big.Int).SetBytes(rawBalance),
		})
	}

	keys := make([]*ngtypes.RegisteredKey, 0)
	c = txn.Bucket(storage.KeyRegistryBucketName).Cursor()
	for addr, entry := c.First(); addr != nil; addr, entry = c.Next() {
		row := &ngtypes.RegisteredKey{Entry: append([]byte{}, entry...)}
		copy(row.Address[:], addr)
		keys = append(keys, row)
	}

	sheet := ngtypes.NewSheet(network, height, blockHash, balances, contracts, keys)
	// carry the native randomness beacon seed so the snapshot reproduces the
	// identical StateRoot; nil (left off the sheet) before the beacon fork
	if b := txn.Bucket(storage.BeaconBucketName); b != nil {
		if v := b.Get(ngtypes.BeaconStateKey); v != nil {
			sheet.Beacon = append([]byte{}, v...)
		}
	}
	return sheet, nil
}

// GenerateSnapshotTxn captures the current state as the sheet of the
// latest block (a checkpoint) inside the given txn, making it servable
// to snapshot-syncing peers
func (state *State) GenerateSnapshotTxn(txn *bbolt.Tx) error {
	sheet, err := DumpSheetTxn(state.Network, txn)
	if err != nil {
		return err
	}

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
	if err := snapshotBucket.Put(snapshotKey(sheet.Height), raw); err != nil {
		return err
	}

	// prune persisted snapshots below the retention window. keys are
	// big-endian, so First()->Next() walks heights in ascending order and
	// the loop can stop at the first survivor
	if sheet.Height > snapshotRetention*ngtypes.BlockCheckRound {
		floor := sheet.Height - snapshotRetention*ngtypes.BlockCheckRound
		c := snapshotBucket.Cursor()
		for k, _ := c.First(); k != nil && binary.BigEndian.Uint64(k) < floor; k, _ = c.Next() {
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

		raw := snapshotBucket.Get(snapshotKey(height))
		if raw == nil {
			// nearest older snapshot: better a conservative floor than
			// an error (pre-persistence dbs, retention gaps). big-endian
			// keys iterate in height order, so the last one <= height is the
			// nearest older
			c := snapshotBucket.Cursor()
			var candidate []byte
			for k, v := c.First(); k != nil && binary.BigEndian.Uint64(k) <= height; k, v = c.Next() {
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
