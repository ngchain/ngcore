package ngstate

import (
	"sync"

	"github.com/c0mm4nd/rlp"
	"github.com/dgraph-io/badger/v3"
	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("sheet")

var (
	numToAccountPrefix  = []byte("nu:")
	addrToBalancePrefix = []byte("ab:")
	addrToNumPrefix     = []byte("an:")
)

// State is a global set of account & txs status
// (nil) --> B0(Prev: S0) --> B1(Prev: S1) -> B2(Prev: S2)
//  init (S0,S0)  -->   (S0,S1)  -->    (S1, S2)
type State struct {
	Network ngtypes.Network

	*badger.DB
	*SnapshotManager

	vms map[ngtypes.AccountNum]*VM
}

// InitStateFromSheet will initialize the state in the given db, with the sheet data
// this func is written for snapshot sync/converging when initializing from non-genesis
// checkpoint
func InitStateFromSheet(db *badger.DB, network ngtypes.Network, sheet *ngtypes.Sheet) *State {
	state := &State{
		DB: db,
		SnapshotManager: &SnapshotManager{
			RWMutex:        sync.RWMutex{},
			heightToHash:   make(map[uint64]string),
			hashToSnapshot: make(map[string]*ngtypes.Sheet),
		},

		vms: make(map[ngtypes.AccountNum]*VM),
	}
	err := state.Update(func(txn *badger.Txn) error {
		return initFromSheet(txn, sheet)
	})
	if err != nil {
		panic(err)
	}

	return state
}

// InitStateFromGenesis will initialize the state in the given db, with the default genesis sheet data
func InitStateFromGenesis(db *badger.DB, network ngtypes.Network) *State {
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
	err := state.Update(func(txn *badger.Txn) error {
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
func initFromSheet(txn *badger.Txn, sheet *ngtypes.Sheet) error {
	for num, account := range sheet.Accounts {
		rawAccount, err := rlp.EncodeToBytes(account)
		if err != nil {
			return err
		}

		err = txn.Set(append(numToAccountPrefix, ngtypes.AccountNum(num).Bytes()...), rawAccount)
		if err != nil {
			return err
		}
	}

	for _, balance := range sheet.Balances {
		err := txn.Set(append(addrToBalancePrefix, balance.Address...), balance.Amount.Bytes())
		if err != nil {
			return err
		}
	}

	return nil
}

// RebuildFromSheet will overwrite a state from the given sheet
func (state *State) RebuildFromSheet(sheet *ngtypes.Sheet) error {
	err := state.DropPrefix(addrToBalancePrefix)
	if err != nil {
		return err
	}
	err = state.DropPrefix(numToAccountPrefix)
	if err != nil {
		return err
	}

	err = state.Update(func(txn *badger.Txn) error {
		return initFromSheet(txn, sheet)
	})
	if err != nil {
		return err
	}

	return nil
}

// RebuildFromBlockStore works for doing converge and remove all
func (state *State) RebuildFromBlockStore() error {
	err := state.DropPrefix(addrToBalancePrefix)
	if err != nil {
		return err
	}
	err = state.DropPrefix(numToAccountPrefix)
	if err != nil {
		return err
	}

	var latestHeight uint64
	err = state.Update(func(txn *badger.Txn) error {
		err := initFromSheet(txn, ngtypes.GetGenesisSheet(state.Network))
		if err != nil {
			return err
		}

		latestHeight, err = ngblocks.GetLatestHeight(txn)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	for h := uint64(0); h <= latestHeight; h++ {
		err = state.Update(func(txn *badger.Txn) error {
			b, err := ngblocks.GetBlockByHeight(txn, h)
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
func (state *State) Upgrade(txn *badger.Txn, block *ngtypes.Block) error {
	err := state.HandleTxs(txn, block.Txs...)
	if err != nil {
		return err
	}

	return nil
}
