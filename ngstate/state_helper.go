package ngstate

import (
	"fmt"
	"math/big"

	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func getAccount(txn *badger.Txn, num ngtypes.AccountNum) (*ngtypes.Account, error) {
	item, err := txn.Get(append(accountPrefix, num.Bytes()...))
	if err == badger.ErrKeyNotFound {
		return nil, err // export the keynotfound
	}
	if err != nil {
		return nil, fmt.Errorf("cannot find account: %s", err)
	}

	rawAcc, err := item.ValueCopy(nil)
	if err != nil {
		return nil, fmt.Errorf("cannot get account: %s", err)
	}

	acc := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawAcc, acc)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func getBalance(txn *badger.Txn, addr ngtypes.Address) (*big.Int, error) {
	item, err := txn.Get(append(addressPrefix, addr...))
	if err == badger.ErrKeyNotFound {
		return big.NewInt(0), nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot find balance: %s", err)
	}

	rawBalance, err := item.ValueCopy(nil)
	if err != nil {
		return nil, fmt.Errorf("cannot get balance: %s", err)
	}

	return new(big.Int).SetBytes(rawBalance), nil
}

func setAccount(txn *badger.Txn, num ngtypes.AccountNum, account *ngtypes.Account) error {
	rawAccount, err := utils.Proto.Marshal(account)
	if err != nil {
		return err
	}
	err = txn.Set(append(accountPrefix, num.Bytes()...), rawAccount)
	if err != nil {
		return fmt.Errorf("cannot set account: %s", err)
	}

	return nil
}

func setBalance(txn *badger.Txn, addr ngtypes.Address, balance *big.Int) error {
	err := txn.Set(append(addressPrefix, addr...), balance.Bytes())
	if err != nil {
		return fmt.Errorf("cannot find balance: %s", err)
	}

	return nil
}

func delAccount(txn *badger.Txn, num ngtypes.AccountNum) error {
	return txn.Delete(append(accountPrefix, num.Bytes()...))
}

func accountExists(txn *badger.Txn, num ngtypes.AccountNum) bool {
	item, err := txn.Get(append(accountPrefix, num.Bytes()...))
	if err != nil {
		return false
	}

	v, _ := item.ValueCopy(nil)

	return v != nil
}
