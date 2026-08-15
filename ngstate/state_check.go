package ngstate

import (
	"math/big"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

var ErrTxrBalanceInsufficient = errors.New("address balance is not sufficient for the tx")

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
			if err := checkGenerate(txn, tx, block.GetHeight()); err != nil {
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

	case ngtypes.CommitTx: // edit
		if err := checkCommit(txn, tx); err != nil {
			return err
		}

	case ngtypes.ActivateTx: // lock
		if err := checkActivate(txn, tx); err != nil {
			return err
		}

	case ngtypes.DeactivateTx: // unlock
		if err := checkDeactivate(txn, tx); err != nil {
			return err
		}

	default:
		return ngtypes.ErrTxTypeInvalid
	}

	return nil
}

// checkGenerate checks the generate tx
func checkGenerate(txn *bbolt.Tx, generateTx *ngtypes.FullTx, blockHeight uint64) error {
	return generateTx.CheckGenerate(blockHeight, keyResolver(txn))
}

// fromWithBalance derives the From address and checks it can afford the
// expense
func fromWithBalance(txn *bbolt.Tx, tx *ngtypes.FullTx, expense *big.Int) (ngtypes.Address, error) {
	from, err := tx.From()
	if err != nil {
		return ngtypes.Address{}, err
	}

	if getBalance(txn, from).Cmp(expense) < 0 {
		return ngtypes.Address{}, ErrTxrBalanceInsufficient
	}

	return from, nil
}

// checkDestroy checks destroy tx: the From address clears its own slot,
// which must exist, be inactive and unreferenced
func checkDestroy(txn *bbolt.Tx, destroyTx *ngtypes.FullTx) error {
	if err := destroyTx.CheckDestroy(keyResolver(txn)); err != nil {
		return err
	}

	from, err := fromWithBalance(txn, destroyTx, destroyTx.TotalExpenditure())
	if err != nil {
		return err
	}

	slot, err := getContract(txn, from)
	if err != nil {
		return err
	}

	if slot.IsActive() {
		return ErrContractActive
	}
	if refs := getRefCount(slot); refs > 0 {
		return errors.Wrapf(ErrContractRefdBy, "%d dependent contract(s)", refs)
	}

	return nil
}

// checkTransaction checks normal transaction tx
func checkTransaction(txn *bbolt.Tx, transactionTx *ngtypes.FullTx) error {
	if err := transactionTx.CheckTransaction(keyResolver(txn)); err != nil {
		return err
	}

	_, err := fromWithBalance(txn, transactionTx, transactionTx.TotalExpenditure())

	return err
}

// checkCommit checks commit tx: a dry-run of the whole patch
// application; the first commit on an address opens its contract slot
func checkCommit(txn *bbolt.Tx, commitTx *ngtypes.FullTx) error {
	if err := commitTx.CheckCommit(keyResolver(txn)); err != nil {
		return err
	}

	from, err := fromWithBalance(txn, commitTx, commitTx.TotalExpenditure())
	if err != nil {
		return err
	}

	baseText := []byte(nil)

	slot, err := getContract(txn, from)
	if err == nil {
		// an active contract is immutable
		if slot.IsActive() {
			return ErrContractActive
		}
		baseText = slot.Source
	}

	editExtra, err := ngtypes.DecodeCommitExtra(commitTx.Extra)
	if err != nil {
		return err
	}

	if _, err := editExtra.Apply(baseText); err != nil {
		return err
	}

	return nil
}

// checkActivate checks activate tx
func checkActivate(txn *bbolt.Tx, activateTx *ngtypes.FullTx) error {
	if err := activateTx.CheckActivate(keyResolver(txn)); err != nil {
		return err
	}

	from, err := fromWithBalance(txn, activateTx, activateTx.TotalExpenditure())
	if err != nil {
		return err
	}

	slot, err := getContract(txn, from)
	if err != nil {
		return err
	}

	if slot.IsActive() {
		return ErrContractActive
	}

	// locking activates the vm, so the contract text must compile, and
	// every declared module dependency must be an active contract
	if len(slot.Source) != 0 {
		deps, err := extractContractDeps(slot.Source)
		if err != nil {
			return err
		}
		for _, depAddr := range deps {
			if depAddr.Equals(from) {
				return ErrDepSelf
			}
			depAcc, err := getContract(txn, depAddr)
			if err != nil {
				return errors.Wrapf(err, "unknown dependency contract %s", depAddr)
			}
			if !depAcc.IsActive() || len(depAcc.Source) == 0 {
				return errors.Wrapf(ErrDepNotActive, "contract %s", depAddr)
			}
		}
	}

	return nil
}

// checkDeactivate checks unactivate tx
func checkDeactivate(txn *bbolt.Tx, deactivateTx *ngtypes.FullTx) error {
	if err := deactivateTx.CheckDeactivate(keyResolver(txn)); err != nil {
		return err
	}

	from, err := fromWithBalance(txn, deactivateTx, deactivateTx.TotalExpenditure())
	if err != nil {
		return err
	}

	slot, err := getContract(txn, from)
	if err != nil {
		return err
	}

	if !slot.IsActive() {
		return ErrContractNotActive
	}

	// a depended-on module cannot deactivate
	if refs := getRefCount(slot); refs > 0 {
		return errors.Wrapf(ErrContractRefdBy, "%d dependent contract(s)", refs)
	}

	return nil
}
