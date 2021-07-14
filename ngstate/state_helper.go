package ngstate

import (
	"math/big"

	"github.com/c0mm4nd/rlp"
	"github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

func getAccountByNum(txn *badger.Txn, num ngtypes.AccountNum) (*ngtypes.Account, error) {
	item, err := txn.Get(append(numToAccountPrefix, num.Bytes()...))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, err // export the keynotfound
	}
	if err != nil {
		return nil, errors.Wrap(err, "cannot find account")
	}

	rawAcc, err := item.ValueCopy(nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get account")
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
	if errors.Is(err, badger.ErrKeyNotFound) {
		return 0, err // export the keynotfound
	}
	if err != nil {
		return 0, errors.Wrap(err, "cannot find account")
	}

	rawNum, err := item.ValueCopy(nil)
	if err != nil {
		return 0, errors.Wrap(err, "cannot get account")
	}

	num := ngtypes.NewNumFromBytes(rawNum)

	return num, nil
}

func getBalance(txn *badger.Txn, addr ngtypes.Address) (*big.Int, error) {
	item, err := txn.Get(append(addrToBalancePrefix, addr...))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return big.NewInt(0), nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "cannot find balance")
	}

	rawBalance, err := item.ValueCopy(nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get balance")
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
		return errors.Wrap(err, "cannot set account")
	}

	return nil
}

func setBalance(txn *badger.Txn, addr ngtypes.Address, balance *big.Int) error {
	err := txn.Set(append(addrToBalancePrefix, addr...), balance.Bytes())
	if err != nil {
		return errors.Wrap(err, "cannot set balance")
	}

	return nil
}

func delAccount(txn *badger.Txn, num ngtypes.AccountNum) error {
	return txn.Delete(append(numToAccountPrefix, num.Bytes()...))
}

func setOwnership(txn *badger.Txn, addr ngtypes.Address, num ngtypes.AccountNum) error {
	err := txn.Set(append(addrToNumPrefix, addr...), num.Bytes())
	if err != nil {
		return errors.Wrap(err, "cannot set ownership: %s")
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
