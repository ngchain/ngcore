package ngstate

import (
	"math/big"

	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// getAccount loads the contract slot of an address; ErrKeyNotFound
// when the address never deployed
func getAccount(txn *bbolt.Tx, addr ngtypes.Address) (*ngtypes.Account, error) {
	contractBucket := txn.Bucket(storage.ContractBucketName)

	rawAcc := contractBucket.Get(addr[:])
	if rawAcc == nil {
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "no contract slot on %s", addr)
	}

	var acc ngtypes.Account
	err := rlp.DecodeBytes(rawAcc, &acc)
	if err != nil {
		return nil, err
	}

	return &acc, nil
}

func accountExists(txn *bbolt.Tx, addr ngtypes.Address) bool {
	return txn.Bucket(storage.ContractBucketName).Get(addr[:]) != nil
}

func setAccount(txn *bbolt.Tx, account *ngtypes.Account) error {
	rawAccount, err := rlp.EncodeToBytes(account)
	if err != nil {
		return err
	}

	contractBucket := txn.Bucket(storage.ContractBucketName)
	err = contractBucket.Put(account.Owner[:], rawAccount)
	if err != nil {
		return errors.Wrap(err, "cannot set account")
	}

	return nil
}

func delAccount(txn *bbolt.Tx, addr ngtypes.Address) error {
	return txn.Bucket(storage.ContractBucketName).Delete(addr[:])
}

func getBalance(txn *bbolt.Tx, addr ngtypes.Address) *big.Int {
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	rawBalance := addr2balBucket.Get(addr[:])
	if rawBalance == nil {
		return big.NewInt(0)
	}

	return new(big.Int).SetBytes(rawBalance)
}

func setBalance(txn *bbolt.Tx, addr ngtypes.Address, balance *big.Int) error {
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	err := addr2balBucket.Put(addr[:], balance.Bytes())
	if err != nil {
		return errors.Wrapf(err, "failed to set balance")
	}

	return nil
}
