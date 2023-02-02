package ngstate

import (
	"math/big"

	"go.etcd.io/bbolt"
	"github.com/ngchain/ngcore/ngtypes"
)

func vmTransfer(txn *bbolt.Tx, from, to, value uint64) error {
	bigValue := new(big.Int).SetUint64(value)

	convener, err := getAccountByNum(txn, ngtypes.AccountNum(from))
	if err != nil {
		return err
	}

	convenerBalance := getBalance(txn, convener.Owner)

	if convenerBalance.Cmp(bigValue) < 0 {
		return ErrTxrBalanceInsufficient
	}
	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, bigValue))
	if err != nil {
		return err
	}

	participant, err := getAccountByNum(txn, ngtypes.AccountNum(to))
	if err != nil {
		return err
	}

	participantBalance := getBalance(txn, participant.Owner)

	err = setBalance(txn, participant.Owner, new(big.Int).Add(
		participantBalance,
		bigValue,
	))
	if err != nil {
		return err
	}

	err = setAccount(txn, ngtypes.AccountNum(from), convener)
	if err != nil {
		return err
	}

	return nil
}
