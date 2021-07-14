package ngstate

import (
	"bytes"
	"encoding/binary"

	"github.com/c0mm4nd/rlp"
	"github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

var ErrTxrBalanceInsufficient = errors.New("account's balance is not sufficient for the tx")

// CheckBlockTxs will check all requirements for txs in block
func CheckBlockTxs(txn *badger.Txn, block *ngtypes.Block) error {
	for i := 0; i < len(block.Txs); i++ {
		tx := block.Txs[i]
		// check tx is signed
		if !tx.IsSigned() {
			return ngtypes.ErrTxUnsigned
		}

		// check the tx's extra size is necessary
		if len(tx.Extra) > ngtypes.TxMaxExtraSize {
			return ngtypes.ErrTxExtraExcess
		}

		switch tx.Type {
		case ngtypes.GenerateTx: // generate
			if err := checkGenerate(txn, tx, block.Header.Height); err != nil {
				return err
			}

		case ngtypes.RegisterTx: // register
			if err := checkRegister(txn, tx); err != nil {
				return err
			}

		case ngtypes.DestroyTx: // destroy
			if err := checkDestroy(txn, tx); err != nil {
				return err
			}

		case ngtypes.TransactTx: // transact
			if err := checkTransaction(txn, tx); err != nil {
				return err
			}

		case ngtypes.AppendTx: // append
			if err := checkAppend(txn, tx); err != nil {
				return err
			}

		case ngtypes.DeleteTx: // delete
			if err := checkDelete(txn, tx); err != nil {
				return err
			}
		default:
			return ngtypes.ErrTxTypeInvalid
		}
	}

	return nil
}

// CheckTx will check the requirements for one tx (except generate tx)
func CheckTx(txn *badger.Txn, tx *ngtypes.Tx) error {
	// check tx is signed
	if !tx.IsSigned() {
		return ngtypes.ErrTxSignInvalid
	}

	// check the tx's extra size is necessary
	if len(tx.Extra) > ngtypes.TxMaxExtraSize {
		return ngtypes.ErrTxExtraExcess
	}

	switch tx.Type {
	case ngtypes.GenerateTx: // generate
		panic("shouldnt check generate tx in this func")

	case ngtypes.RegisterTx: // register
		if err := checkRegister(txn, tx); err != nil {
			return err
		}

	case ngtypes.DestroyTx: // destroy
		if err := checkDestroy(txn, tx); err != nil {
			return err
		}

	case ngtypes.TransactTx: // transact
		if err := checkTransaction(txn, tx); err != nil {
			return err
		}

	case ngtypes.DeleteTx: // delete
		if err := checkDelete(txn, tx); err != nil {
			return err
		}

	case ngtypes.AppendTx: // append
		if err := checkAppend(txn, tx); err != nil {
			return err
		}
	}

	return nil
}

// checkGenerate checks the generate tx
func checkGenerate(txn *badger.Txn, generateTx *ngtypes.Tx, blockHeight uint64) error {
	item, err := txn.Get(append(numToAccountPrefix, generateTx.Convener.Bytes()...))
	if err != nil {
		return errors.Wrapf(err, "cannot find convener %d", generateTx.Convener)
	}

	rawConvener, err := item.ValueCopy(nil)
	if err != nil {
		return errors.Wrapf(err, "cannot get convener account %d", generateTx.Convener)
	}

	var convener ngtypes.Account
	err = rlp.DecodeBytes(rawConvener, convener)
	if err != nil {
		return err
	}

	// check structure and key
	if err := generateTx.CheckGenerate(blockHeight); err != nil {
		return err
	}

	// check rew

	return nil
}

var (
	ErrTxRegExcess = errors.New("one address can only register one accounts")
	ErrTxRegExist  = errors.New("account is already registered by others")
)

// checkRegister checks the register tx
func checkRegister(txn *badger.Txn, registerTx *ngtypes.Tx) error {
	// check structure and key
	if err := registerTx.CheckRegister(); err != nil {
		return err
	}

	// check balance
	payerAddr := registerTx.Participants[0]
	payerBalance, err := getBalance(txn, payerAddr)
	if err != nil {
		return err
	}

	expenditure := registerTx.TotalExpenditure()
	if payerBalance.Cmp(expenditure) < 0 {
		return ErrTxrBalanceInsufficient
	}

	// check existing ownership
	if addrHasAccount(txn, payerAddr) {
		return ErrTxRegExcess
	}

	// check newAccountNum
	newAccountNum := binary.LittleEndian.Uint64(registerTx.Extra)
	if accountNumExists(txn, ngtypes.AccountNum(newAccountNum)) {
		return errors.Wrapf(ErrTxRegExist, "failed to register account@%d", newAccountNum)
	}

	return nil
}

var ErrDestroyAccountContractNotEmpty = errors.New("contract should be empty on destroy tx")

// checkDestroy checks destroy tx
func checkDestroy(txn *badger.Txn, destroyTx *ngtypes.Tx) error {
	convener, err := getAccountByNum(txn, destroyTx.Convener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = destroyTx.CheckDestroy(ngtypes.Address(convener.Owner).PubKey()); err != nil {
		return err
	}

	// check balance
	totalCharge := destroyTx.TotalExpenditure()
	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ErrTxrBalanceInsufficient
	}

	if len(convener.Contract) != 0 {
		return ErrDestroyAccountContractNotEmpty
	}

	// TODO
	// if len(convener.Context) != 0 {
	//	return fmt.Errorf("you should clear your context before destroy")
	// }

	return nil
}

// checkTransaction checks normal transaction tx
func checkTransaction(txn *badger.Txn, transactionTx *ngtypes.Tx) error {
	convener, err := getAccountByNum(txn, transactionTx.Convener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = transactionTx.CheckTransaction(ngtypes.Address(convener.Owner).PubKey()); err != nil {
		return err
	}

	// check balance
	totalCharge := transactionTx.TotalExpenditure()
	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ErrTxrBalanceInsufficient
	}

	return nil
}

var (
	ErrPosOutOfBound = errors.New("pos out of bound")
	ErrLenExcess     = errors.New("length is excess")
	ErrLenInvalid    = errors.New("length is invalid")
)

// checkAppend checks append tx
func checkAppend(txn *badger.Txn, appendTx *ngtypes.Tx) error {
	convener, err := getAccountByNum(txn, appendTx.Convener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = appendTx.CheckAppend(ngtypes.Address(convener.Owner).PubKey()); err != nil {
		return err
	}

	// check balance
	totalCharge := appendTx.TotalExpenditure()
	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ErrTxrBalanceInsufficient
	}

	var appendExtra ngtypes.AppendExtra
	err = rlp.DecodeBytes(appendTx.Extra, &appendExtra)
	if err != nil {
		return err
	}

	if appendExtra.Pos >= uint64(len(convener.Contract)) {
		return ErrPosOutOfBound
	}

	return nil
}

// checkDelete checks delete tx
func checkDelete(txn *badger.Txn, deleteTx *ngtypes.Tx) error {
	convener, err := getAccountByNum(txn, deleteTx.Convener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = deleteTx.CheckDelete(ngtypes.Address(convener.Owner).PubKey()); err != nil {
		return err
	}

	// check balance
	totalCharge := deleteTx.TotalExpenditure()
	convenerBalance, err := getBalance(txn, convener.Owner)
	if err != nil {
		return err
	}

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ErrTxrBalanceInsufficient
	}

	var appendExtra ngtypes.DeleteExtra
	err = rlp.DecodeBytes(deleteTx.Extra, &appendExtra)
	if err != nil {
		return err
	}

	if appendExtra.Pos >= uint64(len(convener.Contract)) {
		return ErrPosOutOfBound
	}

	if appendExtra.Pos+uint64(len(appendExtra.Content)) >= uint64(len(convener.Contract)) {
		return ErrLenExcess
	}

	if !bytes.Equal(
		convener.Contract[int(appendExtra.Pos):int(appendExtra.Pos)+len(appendExtra.Content)],
		appendExtra.Content) {
		return ErrLenInvalid
	}

	return nil
}
