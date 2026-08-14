package ngstate

import (
	"math/big"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// HandleTxs will apply the tx into the state if tx is VALID.
// blockTime is the enclosing block's timestamp, exposed to contracts
func (state *State) HandleTxs(txn *bbolt.Tx, blockTime uint64, txs ...*ngtypes.FullTx) (err error) {
	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		switch tx.Type {
		case ngtypes.InvalidTx:
			return ngtypes.ErrTxTypeInvalid
		case ngtypes.GenerateTx:
			if err := state.handleGenerate(txn, tx); err != nil {
				return err
			}
		case ngtypes.DestroyTx:
			if err := state.handleDestroy(txn, tx); err != nil {
				return err
			}
		case ngtypes.TransactTx:
			if err := state.handleTransaction(txn, tx, blockTime); err != nil {
				return err
			}
		case ngtypes.CommitTx: // commit tx
			if err := state.handleCommit(txn, tx); err != nil {
				return err
			}
		case ngtypes.ActivateTx:
			if err := state.handleActivate(txn, tx, blockTime); err != nil {
				return err
			}
		case ngtypes.DeactivateTx:
			if err := state.handleDeactivate(txn, tx); err != nil {
				return err
			}
		default:
			return errors.Wrapf(ngtypes.ErrTxTypeInvalid, "unknown tx type %d", tx.Type)
		}
	}

	return nil
}

func (state *State) handleGenerate(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	if err := tx.Verify(); err != nil {
		return err
	}

	balance := getBalance(txn, tx.Participants[0])

	err = setBalance(txn, tx.Participants[0], new(big.Int).Add(balance, tx.Values[0]))
	if err != nil {
		return err
	}

	return nil
}

// senderAndCharge verifies the tx, derives its sender and burns the
// expenditure from the sender's balance
func senderAndCharge(txn *bbolt.Tx, tx *ngtypes.FullTx, expense *big.Int) (ngtypes.Address, error) {
	if err := tx.Verify(); err != nil {
		return ngtypes.Address{}, err
	}

	sender, err := tx.Sender()
	if err != nil {
		return ngtypes.Address{}, err
	}

	balance := getBalance(txn, sender)
	if balance.Cmp(expense) < 0 {
		return ngtypes.Address{}, ErrTxrBalanceInsufficient
	}

	if err := setBalance(txn, sender, new(big.Int).Sub(balance, expense)); err != nil {
		return ngtypes.Address{}, err
	}

	return sender, nil
}

// handleDestroy removes the sender's contract slot entirely (contract
// text AND context); the slot must be inactive and unreferenced
func (state *State) handleDestroy(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	sender, err := senderAndCharge(txn, tx, tx.Fee)
	if err != nil {
		return err
	}

	slot, err := getAccount(txn, sender)
	if err != nil {
		return err
	}

	if slot.IsActive() {
		return ErrAccountActive
	}
	if refs := getRefCount(slot); refs > 0 {
		return errors.Wrapf(ErrAccountRefdBy, "%d dependent contract(s)", refs)
	}

	return delAccount(txn, sender)
}

func (state *State) handleTransaction(txn *bbolt.Tx, tx *ngtypes.FullTx, blockTime uint64) (err error) {
	totalValue := big.NewInt(0)
	for i := range tx.Values {
		totalValue.Add(totalValue, tx.Values[i])
	}

	if _, err := senderAndCharge(txn, tx, new(big.Int).Add(tx.Fee, totalValue)); err != nil {
		return err
	}

	for i := range tx.Participants {
		participantBalance := getBalance(txn, tx.Participants[i])

		err = setBalance(txn, tx.Participants[i], new(big.Int).Add(participantBalance, tx.Values[i]))
		if err != nil {
			return err
		}

		if accountExists(txn, tx.Participants[i]) {
			state.runContract(txn, tx.Participants[i], tx, VMEntryOnTx, blockTime)
		}
	}

	return nil
}

// handleCommit applies a whole patch (CommitExtra hunks) onto the sender's
// contract slot atomically. The first edit OPENS the slot — that is
// the namespace purchase, charged at DeployFee on top of the tx fee
func (state *State) handleCommit(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	if err := tx.CheckCommit(); err != nil {
		return err
	}

	sender, err := tx.Sender()
	if err != nil {
		return err
	}

	slot, err := getAccount(txn, sender)
	deploying := err != nil // no slot yet: this edit buys the namespace
	if deploying {
		slot = ngtypes.NewAccount(sender, nil, nil)
	}

	if slot.IsActive() {
		return ErrAccountActive
	}

	expense := new(big.Int).Set(tx.Fee)
	if deploying {
		expense.Add(expense, ngtypes.DeployFee)
	}

	if _, err := senderAndCharge(txn, tx, expense); err != nil {
		return err
	}

	editExtra, err := ngtypes.DecodeCommitExtra(tx.Extra)
	if err != nil {
		return err
	}

	slot.Contract, err = editExtra.Apply(slot.Contract)
	if err != nil {
		return err
	}

	return setAccount(txn, slot)
}

// handleActivate freezes the sender's contract: the body becomes immutable
// and the vm gets active. The optional `init` export runs once here
func (state *State) handleActivate(txn *bbolt.Tx, tx *ngtypes.FullTx, blockTime uint64) (err error) {
	if err := tx.CheckActivate(); err != nil {
		return err
	}

	sender, err := senderAndCharge(txn, tx, tx.Fee)
	if err != nil {
		return err
	}

	slot, err := getAccount(txn, sender)
	if err != nil {
		return err
	}

	if slot.IsActive() {
		return ErrAccountActive
	}

	// locking activates the vm, so the contract text must compile
	if len(slot.Contract) != 0 {
		if _, err := CompileContract(slot.Contract); err != nil {
			return err
		}
	}

	// module dependencies: every imported contract must be active, and
	// each dependee gets a reference pinned until this contract deactivates
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
		if !depAcc.IsActive() || len(depAcc.Contract) == 0 {
			return errors.Wrapf(ErrDepNotActive, "contract %s", depAddr)
		}

		setRefCount(depAcc, getRefCount(depAcc)+1)
		if err := setAccount(txn, depAcc); err != nil {
			return err
		}
	}

	slot.SetActive(true)
	if err := setContractDeps(slot, deps); err != nil {
		return err
	}

	err = setAccount(txn, slot)
	if err != nil {
		return err
	}

	state.runContract(txn, sender, tx, VMEntryOnActivate, blockTime)

	return nil
}

// handleDeactivate disables the vm of the sender's contract and makes the
// body editable again
func (state *State) handleDeactivate(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	if err := tx.CheckDeactivate(); err != nil {
		return err
	}

	sender, err := senderAndCharge(txn, tx, tx.Fee)
	if err != nil {
		return err
	}

	slot, err := getAccount(txn, sender)
	if err != nil {
		return err
	}

	if !slot.IsActive() {
		return ErrAccountNotActive
	}

	// a depended-on module cannot deactivate: its dependents would lose
	// the code they link against
	if refs := getRefCount(slot); refs > 0 {
		return errors.Wrapf(ErrAccountRefdBy, "%d dependent contract(s)", refs)
	}

	// release the references this contract held on its dependencies
	deps, err := getContractDeps(slot)
	if err != nil {
		return err
	}
	for _, depAddr := range deps {
		depAcc, err := getAccount(txn, depAddr)
		if err != nil {
			return err
		}
		if refs := getRefCount(depAcc); refs > 0 {
			setRefCount(depAcc, refs-1)
			if err := setAccount(txn, depAcc); err != nil {
				return err
			}
		}
	}

	slot.SetActive(false)
	if err := setContractDeps(slot, nil); err != nil {
		return err
	}

	return setAccount(txn, slot)
}

// runContract executes the entry export of the address's contract, if
// its slot is locked and has one. A contract failure is final for this
// call (its journal is dropped) but NEVER fails the tx itself: every
// node hits the same result, so consensus is kept
func (state *State) runContract(txn *bbolt.Tx, addr ngtypes.Address, tx *ngtypes.FullTx, entry string, blockTime uint64) {
	account, err := getAccount(txn, addr)
	if err != nil {
		return // no contract slot on this address
	}

	if !account.IsActive() || len(account.Contract) == 0 {
		return
	}

	run := ContractRun{Contract: addr.Bytes(), Entry: entry}

	vm, err := NewVM(txn, account, tx, blockTime)
	if err != nil {
		log.Errorf("failed to build the vm for %s: %v", addr, err)
		run.Error = err.Error()
		recordRun(txn, tx, run)
		return
	}

	err = vm.Run(entry)
	run.GasUsed = vm.cfg.TollStation.GetToll()
	if err != nil {
		if IsExportMissing(err) && entry != VMEntryOnTx {
			return // optional entry (e.g. init) is absent — no run to record
		}

		log.Errorf("contract call %s on %s failed: %v", entry, addr, err)
		run.Error = err.Error()
		recordRun(txn, tx, run)
		return
	}

	run.Ok = true
	run.Events = vm.Events()
	recordRun(txn, tx, run)
}

// recordRun appends the run to the tx's local receipt; receipt failures
// must never fail consensus, so they only log
func recordRun(txn *bbolt.Tx, tx *ngtypes.FullTx, run ContractRun) {
	if err := appendContractRun(txn, tx.GetHash(), run); err != nil {
		log.Errorf("failed to record the receipt of tx %x: %v", tx.GetHash(), err)
	}
}
