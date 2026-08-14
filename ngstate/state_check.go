package ngstate

import (
	"math/big"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
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

		if tx.Type == ngtypes.GenerateTx {
			if err := checkGenerate(tx, block.GetHeight()); err != nil {
				return err
			}
			continue
		}

		if err := CheckTx(txn, tx); err != nil {
			return err
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

	return nil
}

// checkGenerate checks the generate tx
func checkGenerate(generateTx *ngtypes.FullTx, blockHeight uint64) error {
	return generateTx.CheckGenerate(blockHeight)
}

// senderWithBalance derives the sender and checks it can afford the
// expense
func senderWithBalance(txn *bbolt.Tx, tx *ngtypes.FullTx, expense *big.Int) (ngtypes.Address, error) {
	sender, err := tx.Sender()
	if err != nil {
		return ngtypes.Address{}, err
	}

	if getBalance(txn, sender).Cmp(expense) < 0 {
		return ngtypes.Address{}, ErrTxrBalanceInsufficient
	}

	return sender, nil
}

// checkDestroy checks destroy tx: the sender clears its own slot,
// which must exist, be unlocked and unreferenced
func checkDestroy(txn *bbolt.Tx, destroyTx *ngtypes.FullTx) error {
	if err := destroyTx.CheckDestroy(); err != nil {
		return err
	}

	sender, err := senderWithBalance(txn, destroyTx, destroyTx.TotalExpenditure())
	if err != nil {
		return err
	}

	slot, err := getAccount(txn, sender)
	if err != nil {
		return err
	}

	if slot.IsLocked() {
		return ErrAccountLocked
	}
	if refs := getRefCount(slot); refs > 0 {
		return errors.Wrapf(ErrAccountRefdBy, "%d dependent contract(s)", refs)
	}

	return nil
}

// checkTransaction checks normal transaction tx
func checkTransaction(txn *bbolt.Tx, transactionTx *ngtypes.FullTx) error {
	if err := transactionTx.CheckTransaction(); err != nil {
		return err
	}

	_, err := senderWithBalance(txn, transactionTx, transactionTx.TotalExpenditure())

	return err
}

// checkEdit checks edit tx: a dry-run of the whole patch application.
// The first edit on an address opens its contract slot and must carry
// the one-time DeployFee on top
func checkEdit(txn *bbolt.Tx, editTx *ngtypes.FullTx) error {
	if err := editTx.CheckEdit(); err != nil {
		return err
	}

	sender, err := editTx.Sender()
	if err != nil {
		return err
	}

	baseText := []byte(nil)
	expense := new(big.Int).Set(editTx.Fee)

	slot, err := getAccount(txn, sender)
	if err == nil {
		// a locked contract is immutable
		if slot.IsLocked() {
			return ErrAccountLocked
		}
		baseText = slot.Contract
	} else {
		// no slot yet: this edit is the namespace purchase
		expense.Add(expense, ngtypes.DeployFee)
	}

	if getBalance(txn, sender).Cmp(expense) < 0 {
		return ErrTxrBalanceInsufficient
	}

	editExtra, err := ngtypes.DecodeEditExtra(editTx.Extra)
	if err != nil {
		return err
	}

	if _, err := editExtra.Apply(baseText); err != nil {
		return err
	}

	return nil
}

// checkLock checks lock tx
func checkLock(txn *bbolt.Tx, lockTx *ngtypes.FullTx) error {
	if err := lockTx.CheckLock(); err != nil {
		return err
	}

	sender, err := senderWithBalance(txn, lockTx, lockTx.TotalExpenditure())
	if err != nil {
		return err
	}

	slot, err := getAccount(txn, sender)
	if err != nil {
		return err
	}

	if slot.IsLocked() {
		return ErrAccountLocked
	}

	// locking activates the vm, so the contract text must compile, and
	// every declared module dependency must be an active contract
	if len(slot.Contract) != 0 {
		deps, err := extractContractDeps(slot.Contract)
		if err != nil {
			return err
		}
		for _, depAddr := range deps {
			if depAddr.Equals(sender) {
				return ErrDepSelf
			}
			depAcc, err := getAccount(txn, depAddr)
			if err != nil {
				return errors.Wrapf(err, "unknown dependency contract %s", depAddr)
			}
			if !depAcc.IsLocked() || len(depAcc.Contract) == 0 {
				return errors.Wrapf(ErrDepNotActive, "contract %s", depAddr)
			}
		}
	}

	return nil
}

// checkUnlock checks unlock tx
func checkUnlock(txn *bbolt.Tx, unlockTx *ngtypes.FullTx) error {
	if err := unlockTx.CheckUnlock(); err != nil {
		return err
	}

	sender, err := senderWithBalance(txn, unlockTx, unlockTx.TotalExpenditure())
	if err != nil {
		return err
	}

	slot, err := getAccount(txn, sender)
	if err != nil {
		return err
	}

	if !slot.IsLocked() {
		return ErrAccountNotLocked
	}

	// a depended-on module cannot deactivate
	if refs := getRefCount(slot); refs > 0 {
		return errors.Wrapf(ErrAccountRefdBy, "%d dependent contract(s)", refs)
	}

	return nil
}
