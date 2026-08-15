package ngstate

import (
	"sync"

	"github.com/c0mm4nd/rlp"
	logging "github.com/ngchain/zap-log"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var log = logging.Logger("sheet")

// State is a global set of account & txs status
// (nil) --> B0(Prev: S0) --> B1(Prev: S1) -> B2(Prev: S2)
//
//	init (S0,S0)  -->   (S0,S1)  -->    (S1, S2)
type State struct {
	Network ngtypes.Network

	*bbolt.DB
	*SnapshotManager
}

// InitStateFromSheet will initialize the state in the given db, with the sheet data
// this func is written for snapshot sync/converging when initializing from non-genesis
// checkpoint
func InitStateFromSheet(db *bbolt.DB, network ngtypes.Network, sheet *ngtypes.Sheet) *State {
	state := &State{
		DB: db,
		SnapshotManager: &SnapshotManager{
			RWMutex:        sync.RWMutex{},
			heightToHash:   make(map[uint64]string),
			hashToSnapshot: make(map[string]*ngtypes.Sheet),
		},
	}
	err := state.Update(func(txn *bbolt.Tx) error {
		return initFromSheet(txn, sheet)
	})
	if err != nil {
		panic(err)
	}

	return state
}

// InitStateFromGenesis will initialize the state in the given db, with the default genesis sheet data
func InitStateFromGenesis(db *bbolt.DB, network ngtypes.Network) *State {
	state := &State{
		Network: network,
		DB:      db,
		SnapshotManager: &SnapshotManager{
			RWMutex:        sync.RWMutex{},
			heightToHash:   make(map[uint64]string),
			hashToSnapshot: make(map[string]*ngtypes.Sheet),
		},
	}
	err := state.Update(func(txn *bbolt.Tx) error {
		err := initFromSheet(txn, ngtypes.GetGenesisSheet(network))
		if err != nil {
			return err
		}

		err = state.Upgrade(txn, ngtypes.GetGenesisBlock(network))
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		panic(err)
	}

	return state
}

// initFromSheet will overwrite a state from the given sheet
func initFromSheet(txn *bbolt.Tx, sheet *ngtypes.Sheet) error {
	contractBucket := txn.Bucket(storage.ContractBucketName)
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	for _, account := range sheet.Contracts {
		rawAccount, err := rlp.EncodeToBytes(account)
		if err != nil {
			return err
		}

		err = contractBucket.Put(account.Owner[:], rawAccount)
		if err != nil {
			return err
		}
	}

	for _, balance := range sheet.Balances {
		err := addr2balBucket.Put(balance.Address[:], balance.Amount.Bytes())
		if err != nil {
			return err
		}
	}

	keyBucket := txn.Bucket(storage.KeyRegistryBucketName)
	for _, key := range sheet.Keys {
		if err := keyBucket.Put(key.Address[:], key.Entry); err != nil {
			return err
		}
	}

	return nil
}

// RebuildFromSheet will overwrite a state from the given sheet
func (state *State) RebuildFromSheet(sheet *ngtypes.Sheet) error {
	return state.Update(func(txn *bbolt.Tx) error {
		return state.RebuildFromSheetTxn(txn, sheet)
	})
}

// RebuildFromSheetTxn resets the state to the sheet INSIDE the given
// write txn, so a snapshot application can swap the chain and the state
// atomically
func (state *State) RebuildFromSheetTxn(txn *bbolt.Tx, sheet *ngtypes.Sheet) error {
	for _, name := range [][]byte{
		storage.Addr2BalBucketName,
		storage.ContractBucketName,
		storage.KeyRegistryBucketName,
	} {
		if err := txn.DeleteBucket(name); err != nil {
			return err
		}
		if _, err := txn.CreateBucket(name); err != nil {
			return err
		}
	}

	return initFromSheet(txn, sheet)
}

// RebuildFromBlockStore works for doing converge and remove all
func (state *State) RebuildFromBlockStore() error {
	return state.Update(state.RebuildFromBlockStoreTxn)
}

// RebuildFromBlockStoreTxn resets the state and replays the whole
// canonical chain INSIDE the given write txn, so a reorg can swap the
// chain and the state atomically: any failure aborts both
func (state *State) RebuildFromBlockStoreTxn(txn *bbolt.Tx) error {
	for _, name := range [][]byte{
		storage.Addr2BalBucketName,
		storage.ContractBucketName,
		storage.KeyRegistryBucketName,
		storage.ReceiptBucketName, // receipts regenerate with the replay
	} {
		if err := txn.DeleteBucket(name); err != nil {
			return err
		}
		if _, err := txn.CreateBucket(name); err != nil {
			return err
		}
	}

	err := initFromSheet(txn, ngtypes.GetGenesisSheet(state.Network))
	if err != nil {
		return err
	}

	blockBucket := txn.Bucket(storage.BlockBucketName)
	latestHeight, err := ngblocks.GetLatestHeight(blockBucket)
	if err != nil {
		return err
	}

	for h := uint64(0); h <= latestHeight; h++ {
		b, err := ngblocks.GetBlockByHeight(blockBucket, h)
		if err != nil {
			return err
		}

		// the replayed chain may contain blocks which never passed the
		// canonical import path (a reorged-in side branch), so the full
		// tx-level checks (incl. the generate reward amount) run here
		if !b.IsGenesis() {
			if err := CheckBlockTxs(txn, b); err != nil {
				return errors.Wrapf(err, "invalid txs in replayed block@%d", h)
			}
		}

		if err := state.Upgrade(txn, b); err != nil {
			return err
		}
	}

	return nil
}

// Upgrade will apply block's txs on current state
func (state *State) Upgrade(txn *bbolt.Tx, block *ngtypes.FullBlock) error {
	err := state.HandleTxs(txn, block.BlockHeader.Timestamp, block.Txs...)
	if err != nil {
		return err
	}

	return nil
}
