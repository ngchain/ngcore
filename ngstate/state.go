package ngstate

import (
	"sync"

	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("sheet")

var (
	numToAccountPrefix   = []byte("nu:")
	addrTobBalancePrefix = []byte("ab:")
	addrToNumPrefix      = []byte("an:")
)

var state *State

// State is a global set of account & txs status
// (nil) --> B0(Prev: S0) --> B1(Prev: S1) -> B2(Prev: S2)
//  init (S0,S0)  -->   (S0,S1)  -->    (S1, S2)
type State struct {
	*badger.DB
}

// InitStateDB will initialize the state in the given db, with the sheet data
func InitStateDB(db *badger.DB, sheet *ngtypes.Sheet) {
	state = &State{db}
	err := state.Update(func(txn *badger.Txn) error {
		return initFromSheet(txn, sheet)
	})
	if err != nil {
		panic(err)
	}
}

// InitStateFromGenesis will initialize the state in the given db, with the default genesis sheet data
func InitStateFromGenesis(db *badger.DB) {
	state = &State{db}
	err := state.Update(func(txn *badger.Txn) error {
		err := initFromSheet(txn, ngtypes.GenesisSheet)
		if err != nil {
			return err
		}

		err = Upgrade(txn, ngtypes.GetGenesisBlock())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		panic(err)
	}
}

// initFromSheet will overwrite a state from the given sheet
func initFromSheet(txn *badger.Txn, sheet *ngtypes.Sheet) error {
	for num, account := range sheet.Accounts {
		rawAccount, err := utils.Proto.Marshal(account)
		if err != nil {
			return err
		}

		err = txn.Set(append(numToAccountPrefix, ngtypes.AccountNum(num).Bytes()...), rawAccount)
		if err != nil {
			return err
		}
	}

	for addr, balance := range sheet.Anonymous {
		err := txn.Set(append(addrTobBalancePrefix, addr...), balance)
		if err != nil {
			return err
		}
	}

	return nil
}

var regenerateLock sync.Mutex

// Regenerate works for doing fork and remove all
func Regenerate() error {
	regenerateLock.Lock()
	defer regenerateLock.Unlock()

	err := state.DropPrefix(addrTobBalancePrefix)
	if err != nil {
		return err
	}
	err = state.DropPrefix(numToAccountPrefix)
	if err != nil {
		return err
	}

	err = state.Update(func(txn *badger.Txn) error {
		latestHeight, err := ngblocks.GetLatestHeight(txn)
		if err != nil {
			return err
		}

		for h := uint64(0); h <= latestHeight; h++ {
			b, err := ngblocks.GetBlockByHeight(txn, h)
			if err != nil {
				return err
			}

			err = Upgrade(txn, b)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// Upgrade will apply block's txs on current state
func Upgrade(txn *badger.Txn, block *ngtypes.Block) error {
	err := HandleTxs(txn, block.Txs...)
	if err != nil {
		return err
	}

	return nil
}
