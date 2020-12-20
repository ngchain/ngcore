package ngstate

import (
	"fmt"
	"math/big"

	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/ngtypes"
)

func vmTransfer(txn *badger.Txn, from, to, value uint64) error {
	bigValue := new(big.Int).SetUint64(value)

	convener, err := getAccountByNum(txn, ngtypes.AccountNum(from))
	if err != nil {
		return err
	}

	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(bigValue) < 0 {
		return fmt.Errorf("balance is insufficient for transaction")
	}
	err = setBalance(txn, convener.Owner, new(big.Int).Sub(convenerBalance, bigValue))
	if err != nil {
		return err
	}

	participant, err := getAccountByNum(txn, ngtypes.AccountNum(to))
	if err != nil {
		return err
	}

	participantBalance, err := getBalance(txn, participant.Owner)
	if err != nil {
		return err
	}

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
