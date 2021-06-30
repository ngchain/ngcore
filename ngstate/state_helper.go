package ngstate

import (
	"fmt"
	"github.com/c0mm4nd/rlp"
	"math/big"

	"github.com/dgraph-io/badger/v3"
	"github.com/ngchain/ngcore/ngtypes"
)

func getAccountByNum(txn *badger.Txn, num ngtypes.AccountNum) (*ngtypes.Account, error) {
	item, err := txn.Get(append(numToAccountPrefix, num.Bytes()...))
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

	var acc ngtypes.Account
	err = rlp.DecodeBytes(rawAcc, &acc)
	if err != nil {
		return nil, err
	}

	return &acc, nil
}

// DONE: make sure num/addr = 1/1
func getAccountNumByAddr(txn *badger.Txn, addr ngtypes.Address) (ngtypes.AccountNum, error) {
	item, err := txn.Get(append(addrToNumPrefix, addr...))
	if err == badger.ErrKeyNotFound {
		return 0, err // export the keynotfound
	}
	if err != nil {
		return 0, fmt.Errorf("cannot find account: %s", err)
	}

	rawNum, err := item.ValueCopy(nil)
	if err != nil {
		return 0, fmt.Errorf("cannot get account: %s", err)
	}

	num := ngtypes.NewNumFromBytes(rawNum)

	return num, nil
}

func getBalance(txn *badger.Txn, addr ngtypes.Address) (*big.Int, error) {
	item, err := txn.Get(append(addrToBalancePrefix, addr...))
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
	rawAccount, err := rlp.EncodeToBytes(account)
	if err != nil {
		return err
	}
	err = txn.Set(append(numToAccountPrefix, num.Bytes()...), rawAccount)
	if err != nil {
		return fmt.Errorf("cannot set account: %s", err)
	}

	return nil
}

func setBalance(txn *badger.Txn, addr ngtypes.Address, balance *big.Int) error {
	err := txn.Set(append(addrToBalancePrefix, addr...), balance.Bytes())
	if err != nil {
		return fmt.Errorf("cannot set balance: %s", err)
	}

	return nil
}

func delAccount(txn *badger.Txn, num ngtypes.AccountNum) error {
	return txn.Delete(append(numToAccountPrefix, num.Bytes()...))
}

func setOwnership(txn *badger.Txn, addr ngtypes.Address, num ngtypes.AccountNum) error {
	err := txn.Set(append(addrToNumPrefix, addr...), num.Bytes())
	if err != nil {
		return fmt.Errorf("cannot set ownership: %s", err)
	}

	return nil
}

func delOwnership(txn *badger.Txn, addr ngtypes.Address) error {
	return txn.Delete(append(addrToNumPrefix, addr...))
}

func accountNumExists(txn *badger.Txn, num ngtypes.AccountNum) bool {
	item, err := txn.Get(append(numToAccountPrefix, num.Bytes()...))
	if err != nil {
		return false
	}

	v, _ := item.ValueCopy(nil)

	return v != nil
}

func addrHasAccount(txn *badger.Txn, addr ngtypes.Address) bool {
	item, err := txn.Get(append(addrToNumPrefix, addr...))
	if err != nil {
		return false
	}

	v, _ := item.ValueCopy(nil)

	return v != nil
}
