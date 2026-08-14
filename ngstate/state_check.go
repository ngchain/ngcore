package ngstate

import (
	"encoding/binary"

	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var ErrTxrBalanceInsufficient = errors.New("account's balance is not sufficient for the tx")

// CheckBlockTxs will check all requirements for txs in block
func CheckBlockTxs(txn *bbolt.Tx, block *ngtypes.FullBlock) error {
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
			if err := checkGenerate(txn, tx, block.GetHeight()); err != nil {
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

		case ngtypes.EditTx: // edit
			if err := checkEdit(txn, tx); err != nil {
				return err
			}

		case ngtypes.LockTx: // lock
			if err := checkLock(txn, tx); err != nil {
				return err
			}

		case ngtypes.UnlockTx: // unlock
			if err := checkUnlock(txn, tx); err != nil {
				return err
			}
		default:
			return ngtypes.ErrTxTypeInvalid
		}
	}

	return nil
}

// CheckTx will check the requirements for one tx (except generate tx)
func CheckTx(txn *bbolt.Tx, tx *ngtypes.FullTx) error {
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

	case ngtypes.EditTx: // edit
		if err := checkEdit(txn, tx); err != nil {
			return err
		}

	case ngtypes.LockTx: // lock
		if err := checkLock(txn, tx); err != nil {
			return err
		}

	case ngtypes.UnlockTx: // unlock
		if err := checkUnlock(txn, tx); err != nil {
			return err
		}
	}

	return nil
}

// checkGenerate checks the generate tx
func checkGenerate(txn *bbolt.Tx, generateTx *ngtypes.FullTx, blockHeight uint64) error {
	num2accBucket := txn.Bucket(storage.Num2AccBucketName)

	rawConvener := num2accBucket.Get(generateTx.Convener.Bytes())
	if rawConvener == nil {
		return errors.Wrapf(storage.ErrKeyNotFound, "cannot get convener account %d", generateTx.Convener)
	}

	var convener ngtypes.Account
	err := rlp.DecodeBytes(rawConvener, &convener)
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
func checkRegister(txn *bbolt.Tx, registerTx *ngtypes.FullTx) error {
	// check structure and key
	if err := registerTx.CheckRegister(); err != nil {
		return err
	}

	// check balance
	payerAddr := registerTx.Participants[0]
	payerBalance := getBalance(txn, payerAddr)

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
func checkDestroy(txn *bbolt.Tx, destroyTx *ngtypes.FullTx) error {
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
	convenerBalance := getBalance(txn, convener.Owner)

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ErrTxrBalanceInsufficient
	}

	if len(convener.Contract) != 0 {
		return ErrDestroyAccountContractNotEmpty
	}

	// an active (locked) contract may be depended on by others: it must
	// be unlocked first, which also re-enables clearing the contract
	if convener.IsLocked() {
		return ErrAccountLocked
	}
	if refs := getRefCount(convener); refs > 0 {
		return errors.Wrapf(ErrAccountRefdBy, "%d dependent contract(s)", refs)
	}

	return nil
}

// checkTransaction checks normal transaction tx
func checkTransaction(txn *bbolt.Tx, transactionTx *ngtypes.FullTx) error {
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
	convenerBalance := getBalance(txn, convener.Owner)

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ErrTxrBalanceInsufficient
	}

	return nil
}

// checkEdit checks edit tx: a dry-run of the whole patch application
func checkEdit(txn *bbolt.Tx, editTx *ngtypes.FullTx) error {
	convener, err := getAccountByNum(txn, editTx.Convener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = editTx.CheckEdit(ngtypes.Address(convener.Owner).PubKey()); err != nil {
		return err
	}

	// a locked contract is immutable
	if convener.IsLocked() {
		return ErrAccountLocked
	}

	// check balance
	totalCharge := editTx.TotalExpenditure()
	convenerBalance := getBalance(txn, convener.Owner)

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ErrTxrBalanceInsufficient
	}

	editExtra, err := ngtypes.DecodeEditExtra(editTx.Extra)
	if err != nil {
		return err
	}

	if _, err := editExtra.Apply(convener.Contract); err != nil {
		return err
	}

	return nil
}

// checkLock checks lock tx
func checkLock(txn *bbolt.Tx, lockTx *ngtypes.FullTx) error {
	convener, err := getAccountByNum(txn, lockTx.Convener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = lockTx.CheckLock(ngtypes.Address(convener.Owner).PubKey()); err != nil {
		return err
	}

	if convener.IsLocked() {
		return ErrAccountLocked
	}

	// a non-empty lock extra must be a valid, free (or own) name
	if len(lockTx.Extra) != 0 {
		name := string(lockTx.Extra)
		if !validContractName(name) {
			return errors.Wrapf(ErrNameInvalid, "%q", name)
		}
		if existing, err := getNumByName(txn, convener.Owner, name); err == nil && existing != uint64(lockTx.Convener) {
			return errors.Wrapf(ErrNameTaken, "%s -> account %d", name, existing)
		}
	}

	// locking activates the vm, so the contract text must compile, and
	// every declared module dependency must be an active contract
	if len(convener.Contract) != 0 {
		deps, err := extractContractDeps(txn, convener.Contract)
		if err != nil {
			return err
		}
		for _, num := range deps {
			if num == uint64(lockTx.Convener) {
				return ErrDepSelf
			}
			depAcc, err := getAccountByNum(txn, ngtypes.AccountNum(num))
			if err != nil {
				return errors.Wrapf(err, "unknown dependency contract %d", num)
			}
			if !depAcc.IsLocked() || len(depAcc.Contract) == 0 {
				return errors.Wrapf(ErrDepNotActive, "contract %d", num)
			}
		}
	}

	// check balance
	totalCharge := lockTx.TotalExpenditure()
	convenerBalance := getBalance(txn, convener.Owner)

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ErrTxrBalanceInsufficient
	}

	return nil
}

// checkUnlock checks unlock tx
func checkUnlock(txn *bbolt.Tx, unlockTx *ngtypes.FullTx) error {
	convener, err := getAccountByNum(txn, unlockTx.Convener)
	if err != nil {
		return err
	}

	// check structure and key
	if err = unlockTx.CheckUnlock(ngtypes.Address(convener.Owner).PubKey()); err != nil {
		return err
	}

	if !convener.IsLocked() {
		return ErrAccountNotLocked
	}

	// a depended-on module cannot deactivate
	if refs := getRefCount(convener); refs > 0 {
		return errors.Wrapf(ErrAccountRefdBy, "%d dependent contract(s)", refs)
	}

	// check balance
	totalCharge := unlockTx.TotalExpenditure()
	convenerBalance := getBalance(txn, convener.Owner)

	if convenerBalance.Cmp(totalCharge) < 0 {
		return ErrTxrBalanceInsufficient
	}

	return nil
}
