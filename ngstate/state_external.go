package ngstate

import (
	"bytes"
	"math/big"

	"github.com/dgraph-io/badger/v2"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// ToSheet will conclude a sheet which has all status of all accounts & keys(if balance not nil)
func (state *State) ToSheet() *ngtypes.Sheet {
	var prevBlockHash []byte
	accounts := make(map[uint64]*ngtypes.Account)
	anonymous := make(map[string][]byte)
	err := state.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("hash"))
		if err != nil {
			return err
		}

		prevBlockHash = item.KeyCopy(nil)

		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(numToAccountPrefix); it.ValidForPrefix(numToAccountPrefix); it.Next() {
			item := it.Item()
			rawAccount, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			var account ngtypes.Account
			err = utils.Proto.Unmarshal(rawAccount, &account)
			if err != nil {
				return err
			}

			accounts[account.Num] = &account
		}

		it = txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(addrTobBalancePrefix); it.ValidForPrefix(addrTobBalancePrefix); it.Next() {
			item := it.Item()
			addr := item.KeyCopy(nil)
			rawBalance, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			anonymous[base58.FastBase58Encoding(addr)] = rawBalance
		}

		return nil
	})
	if err != nil {
		panic(err)
	}

	return ngtypes.NewSheet(prevBlockHash, accounts, anonymous)
}

// GetBalanceByNum get the balance of account by the account's num
func (state *State) GetBalanceByNum(num uint64) (*big.Int, error) {
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

// GetBalanceByAddress get the balance of account by the account's address
func (state *State) GetBalanceByAddress(address ngtypes.Address) (*big.Int, error) {
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

// AccountIsRegistered checks whether the account is registered in state
func (state *State) AccountIsRegistered(num uint64) bool {
	var exists = true // block register action by default

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
			err = utils.Proto.Unmarshal(rawAccount, &acc)
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
