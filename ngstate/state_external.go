package ngstate

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/c0mm4nd/rlp"

	"github.com/dgraph-io/badger/v3"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
)

// GetTotalBalanceByNum get the balance of account by the account's num
func (state *State) GetTotalBalanceByNum(num uint64) (*big.Int, error) {
	var balance *big.Int

	err := state.View(func(txn *badger.Txn) error {
		account, err := getAccountByNum(txn, ngtypes.AccountNum(num))
		if err != nil {
			return err
		}

		addr := account.Owner
		balance, err = getBalance(txn, addr)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return balance, nil
}

// GetTotalBalanceByAddress get the total balance of account by the account's address
func (state *State) GetTotalBalanceByAddress(address ngtypes.Address) (*big.Int, error) {
	var balance *big.Int

	err := state.View(func(txn *badger.Txn) error {
		var err error
		balance, err = getBalance(txn, address)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return balance, nil
}

// GetMatureBalanceByNum get the balance of account by the account's num
func (state *State) GetMatureBalanceByNum(num uint64) (*big.Int, error) {
	var balance *big.Int

	err := state.View(func(txn *badger.Txn) error {
		account, err := getAccountByNum(txn, ngtypes.AccountNum(num))
		if err != nil {
			return err
		}

		addr := ngtypes.Address(account.Owner)

		currentHeight, err := ngblocks.GetLatestHeight(txn)
		if err != nil {
			return err
		}

		matureSnapshot := state.GetSnapshotByHeight(ngtypes.GetMatureHeight(currentHeight))
		if matureSnapshot == nil {
			return fmt.Errorf("cannot find the mature snapshot") // abnormal
		}

		for i := range matureSnapshot.Balances {
			if bytes.Equal(matureSnapshot.Balances[i].Address, addr) {
				balance = matureSnapshot.Balances[i].Amount
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return balance, nil
}

// GetMatureBalanceByAddress get the locked balance of account by the account's address
func (state *State) GetMatureBalanceByAddress(address ngtypes.Address) (*big.Int, error) {
	var balance *big.Int

	err := state.View(func(txn *badger.Txn) error {
		var err error

		currentHeight, err := ngblocks.GetLatestHeight(txn)
		if err != nil {
			return err
		}

		matureSnapshot := state.GetSnapshotByHeight(ngtypes.GetMatureHeight(currentHeight))
		if matureSnapshot == nil {
			return fmt.Errorf("cannot find the mature snapshot") // abnormal
		}

		for i := range matureSnapshot.Balances {
			if bytes.Equal(matureSnapshot.Balances[i].Address, address) {
				balance = matureSnapshot.Balances[i].Amount
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return balance, nil
}

// AccountIsRegistered checks whether the account is registered in state
func (state *State) AccountIsRegistered(num uint64) bool {
	exists := true // block register action by default

	_ = state.View(func(txn *badger.Txn) error {
		exists = accountNumExists(txn, ngtypes.AccountNum(num))

		return nil
	})

	return exists
}

// GetAccountByNum returns an ngtypes.Account obj by the account's number
func (state *State) GetAccountByNum(num uint64) (account *ngtypes.Account, err error) {
	err = state.View(func(txn *badger.Txn) error {
		account, err = getAccountByNum(txn, ngtypes.AccountNum(num))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return account, nil
}

// GetAccountByAddress returns an ngtypes.Account obj by the account's address
// this is a heavy action, so dont called by any internal part like p2p and consensus
func (state *State) GetAccountByAddress(address ngtypes.Address) (*ngtypes.Account, error) {
	var account *ngtypes.Account
	err := state.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(numToAccountPrefix); it.ValidForPrefix(numToAccountPrefix); it.Next() {
			item := it.Item()
			rawAccount, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			var acc ngtypes.Account
			err = rlp.DecodeBytes(rawAccount, &acc)
			if err != nil {
				return err
			}

			if bytes.Equal(address, acc.Owner) {
				account = &acc
				return nil
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return account, nil
}
