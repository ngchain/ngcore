package ngstate

import (
	"math/big"

	"github.com/c0mm4nd/dbolt"
	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

func getAccountByNum(txn *dbolt.Tx, num ngtypes.AccountNum) (*ngtypes.Account, error) {
	num2accBucket := txn.Bucket(storage.Num2AccBucketName)

	rawAcc := num2accBucket.Get(num.Bytes())
	if rawAcc != nil {
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "cannot find account %d", num)
	}

	var acc ngtypes.Account
	err := rlp.DecodeBytes(rawAcc, &acc)
	if err != nil {
		return nil, err
	}

	return &acc, nil
}

// DONE: make sure num/addr = 1/1
func getAccountNumByAddr(txn *dbolt.Tx, addr ngtypes.Address) (ngtypes.AccountNum, error) {
	addr2numBucket := txn.Bucket(storage.Addr2NumBucketName)

	rawNum := addr2numBucket.Get(addr[:])
	if rawNum == nil {
		return 0, errors.Wrapf(storage.ErrKeyNotFound, "cannot find %s's account", addr)
	}

	num := ngtypes.NewNumFromBytes(rawNum)

	return num, nil
}

func getBalance(txn *dbolt.Tx, addr ngtypes.Address) *big.Int {
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	rawBalance := addr2balBucket.Get(addr[:])
	if rawBalance == nil {
		return big.NewInt(0)
	}

	return new(big.Int).SetBytes(rawBalance)
}

func setAccount(txn *dbolt.Tx, num ngtypes.AccountNum, account *ngtypes.Account) error {
	rawAccount, err := rlp.EncodeToBytes(account)
	if err != nil {
		return err
	}

	num2accBucket := txn.Bucket(storage.Num2AccBucketName)
	err = num2accBucket.Put(num.Bytes(), rawAccount)
	if err != nil {
		return errors.Wrap(err, "cannot set account")
	}

	return nil
}

func setBalance(txn *dbolt.Tx, addr ngtypes.Address, balance *big.Int) error {
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	err := addr2balBucket.Put(addr[:], balance.Bytes())
	if err != nil {
		return errors.Wrapf(err, "failed to set balance")
	}

	return nil
}

func delAccount(txn *dbolt.Tx, num ngtypes.AccountNum) error {
	num2accBucket := txn.Bucket(storage.Num2AccBucketName)

	return num2accBucket.Delete(num.Bytes())
}

func setOwnership(txn *dbolt.Tx, addr ngtypes.Address, num ngtypes.AccountNum) error {
	addr2numBucket := txn.Bucket(storage.Addr2NumBucketName)

	err := addr2numBucket.Put(addr[:], num.Bytes())
	if err != nil {
		return errors.Wrap(err, "cannot set ownership: %s")
	}

	return nil
}

func delOwnership(txn *dbolt.Tx, addr ngtypes.Address) error {
	addr2numBucket := txn.Bucket(storage.Addr2NumBucketName)

	return addr2numBucket.Delete(addr[:])
}

func accountNumExists(txn *dbolt.Tx, num ngtypes.AccountNum) bool {
	num2accBucket := txn.Bucket(storage.Num2AccBucketName)

	return num2accBucket.Get(num.Bytes()) != nil
}

func addrHasAccount(txn *dbolt.Tx, addr ngtypes.Address) bool {
	addr2numBucket := txn.Bucket(storage.Addr2NumBucketName)

	return addr2numBucket.Get(addr[:]) != nil
}
