package ngstate

import (
	"sync"

	"github.com/c0mm4nd/dbolt"
	"github.com/c0mm4nd/rlp"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var log = logging.Logger("sheet")

// State is a global set of account & txs status
// (nil) --> B0(Prev: S0) --> B1(Prev: S1) -> B2(Prev: S2)
//  init (S0,S0)  -->   (S0,S1)  -->    (S1, S2)
type State struct {
	Network ngtypes.Network

	*dbolt.DB
	*SnapshotManager

	vms map[ngtypes.AccountNum]*VM
}

// InitStateFromSheet will initialize the state in the given db, with the sheet data
// this func is written for snapshot sync/converging when initializing from non-genesis
// checkpoint
func InitStateFromSheet(db *dbolt.DB, network ngtypes.Network, sheet *ngtypes.Sheet) *State {
	state := &State{
		DB: db,
		SnapshotManager: &SnapshotManager{
			RWMutex:        sync.RWMutex{},
			heightToHash:   make(map[uint64]string),
			hashToSnapshot: make(map[string]*ngtypes.Sheet),
		},

		vms: make(map[ngtypes.AccountNum]*VM),
	}
	err := state.Update(func(txn *dbolt.Tx) error {
		return initFromSheet(txn, sheet)
	})
	if err != nil {
		panic(err)
	}

	return state
}

// InitStateFromGenesis will initialize the state in the given db, with the default genesis sheet data
func InitStateFromGenesis(db *dbolt.DB, network ngtypes.Network) *State {
	state := &State{
		Network: network,
		DB:      db,
		SnapshotManager: &SnapshotManager{
			RWMutex:        sync.RWMutex{},
			heightToHash:   make(map[uint64]string),
			hashToSnapshot: make(map[string]*ngtypes.Sheet),
		},
		vms: make(map[ngtypes.AccountNum]*VM),
	}
	err := state.Update(func(txn *dbolt.Tx) error {
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
func initFromSheet(txn *dbolt.Tx, sheet *ngtypes.Sheet) error {
	num2accBucket := txn.Bucket(storage.Num2AccBucketName)
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	for num, account := range sheet.Accounts {
		rawAccount, err := rlp.EncodeToBytes(account)
		if err != nil {
			return err
		}

		err = num2accBucket.Put(ngtypes.AccountNum(num).Bytes(), rawAccount)
		if err != nil {
			return err
		}
	}

	for _, balance := range sheet.Balances {
		err := addr2balBucket.Put(balance.Address, balance.Amount.Bytes())
		if err != nil {
			return err
		}
	}

	return nil
}

// RebuildFromSheet will overwrite a state from the given sheet
func (state *State) RebuildFromSheet(sheet *ngtypes.Sheet) error {
	if err := state.Update(func(txn *dbolt.Tx) error {
		err := txn.DeleteBucket(storage.Addr2NumBucketName)
		if err != nil {
			return err
		}
		err = txn.DeleteBucket(storage.Addr2BalBucketName)
		if err != nil {
			return err
		}
		err = txn.DeleteBucket(storage.Num2AccBucketName)
		if err != nil {
			return err
		}
		return initFromSheet(txn, sheet)
	}); err != nil {
		return err
	}

	return nil
}

// RebuildFromBlockStore works for doing converge and remove all
func (state *State) RebuildFromBlockStore() error {

	var latestHeight uint64
	err := state.Update(func(txn *dbolt.Tx) error {
		err := txn.DeleteBucket(storage.Addr2NumBucketName)
		if err != nil {
			return err
		}
		err = txn.DeleteBucket(storage.Addr2BalBucketName)
		if err != nil {
			return err
		}
		err = txn.DeleteBucket(storage.Num2AccBucketName)
		if err != nil {
			return err
		}

		err = initFromSheet(txn, ngtypes.GetGenesisSheet(state.Network))
		if err != nil {
			return err
		}

		blockBucket := txn.Bucket(storage.BlockBucketName)
		latestHeight, err = ngblocks.GetLatestHeight(blockBucket)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	for h := uint64(0); h <= latestHeight; h++ {
		err = state.Update(func(txn *dbolt.Tx) error {
			blockBucket := txn.Bucket(storage.BlockBucketName)
			b, err := ngblocks.GetBlockByHeight(blockBucket, h)
			if err != nil {
				return err
			}

			err = state.Upgrade(txn, b)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Upgrade will apply block's txs on current state
func (state *State) Upgrade(txn *dbolt.Tx, block *ngtypes.FullBlock) error {
	err := state.HandleTxs(txn, block.Txs...)
	if err != nil {
		return err
	}

	return nil
}
