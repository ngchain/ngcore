package ngstate

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// GetContractTxn reads a contract slot within a caller-held txn — used by
// whole-state historical reads that already own a scratch txn
func GetContractTxn(txn *bbolt.Tx, addr ngtypes.Address) (*ngtypes.Contract, error) {
	return getContract(txn, addr)
}

// ReconstructAt rebuilds the FULL state as of `height` in an ISOLATED
// throwaway db, then runs fn against it. Reconstruction seeds from the
// nearest snapshot at or below `height` (genesis at worst) and replays the
// blocks in between. It never touches live state and never holds the live
// write lock — a long-lived live READ txn feeds the blocks while a separate
// scratch WRITE txn absorbs them — so historical whole-state reads
// (callContract / getSheet at a past height) cannot stall block import.
//
// Works on any node that still has the blocks in range (archive not
// required); it errors cleanly when a block below the node's origin is
// missing (a snapshot-synced node querying pre-checkpoint history).
func (state *State) ReconstructAt(height uint64, fn func(txn *bbolt.Tx) error) error {
	var tip uint64
	if err := state.View(func(txn *bbolt.Tx) error {
		var err error
		tip, err = ngblocks.GetLatestHeight(txn.Bucket(storage.BlockBucketName))
		return err
	}); err != nil {
		return err
	}
	if height > tip {
		return errors.Errorf("height %d is above the chain tip %d", height, tip)
	}

	// base state: the nearest snapshot at or below height (genesis at worst)
	sheet := state.GetSnapshotByHeight(height)
	if sheet == nil {
		return errors.Errorf("no base snapshot at or below height %d", height)
	}

	// scratch db in a temp dir, removed on return
	dir, err := os.MkdirTemp("", "ngscratch-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	sdb, err := bbolt.Open(filepath.Join(dir, "state.db"), 0o600, nil)
	if err != nil {
		return err
	}
	defer func() { _ = sdb.Close() }()
	storage.InitDB(sdb)

	scratch := &State{
		Network: state.Network,
		DB:      sdb,
		// Archive off: no changeset capture in a throwaway reconstruction
		SnapshotManager: &SnapshotManager{
			RWMutex:        sync.RWMutex{},
			heightToHash:   make(map[uint64]string),
			hashToSnapshot: make(map[string]*ngtypes.Sheet),
		},
	}

	// one live read txn feeds the blocks (concurrent with writers), one
	// scratch write txn absorbs them — different dbs, no deadlock
	return state.View(func(livetxn *bbolt.Tx) error {
		blockBucket := livetxn.Bucket(storage.BlockBucketName)
		return scratch.Update(func(stxn *bbolt.Tx) error {
			if err := initFromSheet(stxn, sheet); err != nil {
				return err
			}
			for h := sheet.Height + 1; h <= height; h++ {
				block, err := ngblocks.GetBlockByHeight(blockBucket, h)
				if err != nil {
					return errors.Wrapf(err, "reconstruct: missing block@%d", h)
				}
				if err := scratch.Upgrade(stxn, block); err != nil {
					return err
				}
			}
			return fn(stxn)
		})
	})
}
