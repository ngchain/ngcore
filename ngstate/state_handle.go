package ngstate

import (
	"math/big"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// blockGas tracks the remaining contract-execution budget of ONE
// block: deterministic across nodes, so every replica skips the same
// runs once the block budget drains
type blockGas struct {
	remaining uint64
}

// HandleTxs will apply the tx into the state if tx is VALID.
// blockTime is the enclosing block's timestamp, exposed to contracts
func (state *State) HandleTxs(txn *bbolt.Tx, blockTime uint64, txs ...*ngtypes.FullTx) (err error) {
	gas := &blockGas{remaining: ngtypes.MaxBlockGas}

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
			if err := state.handleTransaction(txn, tx, blockTime, gas); err != nil {
				return err
			}
		case ngtypes.CommitTx: // commit tx
			if err := state.handleCommit(txn, tx); err != nil {
				return err
			}
		case ngtypes.ActivateTx:
			if err := state.handleActivate(txn, tx, blockTime, gas); err != nil {
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
	if err := tx.Verify(keyResolver(txn)); err != nil {
		return err
	}

	if err := registerPubKey(txn, state.cs, tx); err != nil {
		return err
	}

	balance := getBalance(txn, tx.To)

	err = setBalance(txn, state.cs, tx.To, new(big.Int).Add(balance, tx.Value))
	if err != nil {
		return err
	}

	return nil
}

// chargeFrom verifies the tx, derives its From address and burns the
// expenditure from the From address's balance
func chargeFrom(txn *bbolt.Tx, rec *changeset, tx *ngtypes.FullTx, expense *big.Int) (ngtypes.Address, error) {
	if err := tx.Verify(keyResolver(txn)); err != nil {
		return ngtypes.Address{}, err
	}

	if err := registerPubKey(txn, rec, tx); err != nil {
		return ngtypes.Address{}, err
	}

	from, err := tx.From()
	if err != nil {
		return ngtypes.Address{}, err
	}

	balance := getBalance(txn, from)
	if balance.Cmp(expense) < 0 {
		return ngtypes.Address{}, ErrTxrBalanceInsufficient
	}

	if err := setBalance(txn, rec, from, new(big.Int).Sub(balance, expense)); err != nil {
		return ngtypes.Address{}, err
	}

	return from, nil
}

// handleDestroy removes the sender's own contract slot entirely (contract
// text AND context); the slot must be inactive and unreferenced
func (state *State) handleDestroy(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	from, err := chargeFrom(txn, state.cs, tx, tx.Fee)
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

	return delContract(txn, state.cs, from)
}

func (state *State) handleTransaction(txn *bbolt.Tx, tx *ngtypes.FullTx, blockTime uint64, gas *blockGas) (err error) {
	if _, err := chargeFrom(txn, state.cs, tx, tx.TotalExpenditure()); err != nil {
		return err
	}

	toBalance := getBalance(txn, tx.To)
	if err := setBalance(txn, state.cs, tx.To, new(big.Int).Add(toBalance, tx.Value)); err != nil {
		return err
	}

	if contractExists(txn, tx.To) {
		state.runContract(txn, tx.To, tx, VMEntryOnTx, blockTime, gas)
	}

	return nil
}

// handleCommit applies a whole patch (CommitExtra hunks) onto the
// sender's contract slot atomically; the first commit OPENS the slot
func (state *State) handleCommit(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	if err := tx.CheckCommit(keyResolver(txn)); err != nil {
		return err
	}

	from, err := tx.From()
	if err != nil {
		return err
	}

	slot, err := getContract(txn, from)
	if err != nil { // no slot yet: this commit opens the namespace
		slot = ngtypes.NewContract(from, nil, nil)
	}

	if slot.IsActive() {
		return ErrContractActive
	}

	if _, err := chargeFrom(txn, state.cs, tx, tx.TotalExpenditure()); err != nil {
		return err
	}

	newSource, err := ngtypes.DecodeCommitCode(tx.Extra)
	if err != nil {
		return err
	}
	if len(newSource) > ngtypes.MaxContractSourceSize {
		return errors.Wrapf(ErrSourceTooLarge, "%d bytes exceed the cap %d",
			len(newSource), ngtypes.MaxContractSourceSize)
	}
	slot.Source = newSource

	return setContract(txn, state.cs, slot)
}

// handleActivate freezes the From address's contract: the body becomes immutable
// and the vm gets active. The optional `init` export runs once here
func (state *State) handleActivate(txn *bbolt.Tx, tx *ngtypes.FullTx, blockTime uint64, gas *blockGas) (err error) {
	if err := tx.CheckActivate(keyResolver(txn)); err != nil {
		return err
	}

	from, err := chargeFrom(txn, state.cs, tx, tx.Fee)
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

	// activating turns the vm on, so the contract text must compile
	if len(slot.Source) != 0 {
		if _, err := LoadContractWasm(slot.Source); err != nil {
			return err
		}
	}

	// module dependencies: every imported contract must be active, and
	// each dependee gets a reference pinned until this contract deactivates
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

		setRefCount(depAcc, getRefCount(depAcc)+1)
		if err := setContract(txn, state.cs, depAcc); err != nil {
			return err
		}
	}

	slot.SetActive(true)
	if err := setContractDeps(slot, deps); err != nil {
		return err
	}

	err = setContract(txn, state.cs, slot)
	if err != nil {
		return err
	}

	state.runContract(txn, from, tx, VMEntryOnActivate, blockTime, gas)

	return nil
}

// handleDeactivate disables the vm of the From address's contract and makes the
// body editable again
func (state *State) handleDeactivate(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	if err := tx.CheckDeactivate(keyResolver(txn)); err != nil {
		return err
	}

	from, err := chargeFrom(txn, state.cs, tx, tx.Fee)
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

	// a depended-on module cannot deactivate: its dependents would lose
	// the code they link against
	if refs := getRefCount(slot); refs > 0 {
		return errors.Wrapf(ErrContractRefdBy, "%d dependent contract(s)", refs)
	}

	// release the references this contract held on its dependencies
	deps, err := getContractDeps(slot)
	if err != nil {
		return err
	}
	for _, depAddr := range deps {
		depAcc, err := getContract(txn, depAddr)
		if err != nil {
			return err
		}
		if refs := getRefCount(depAcc); refs > 0 {
			setRefCount(depAcc, refs-1)
			if err := setContract(txn, state.cs, depAcc); err != nil {
				return err
			}
		}
	}

	slot.SetActive(false)
	if err := setContractDeps(slot, nil); err != nil {
		return err
	}

	return setContract(txn, state.cs, slot)
}

// runContract executes the entry export of the address's contract, if
// its slot is locked and has one. A contract failure is final for this
// call (its journal is dropped) but NEVER fails the tx itself: every
// node hits the same result, so consensus is kept
func (state *State) runContract(txn *bbolt.Tx, addr ngtypes.Address, tx *ngtypes.FullTx, entry string, blockTime uint64, gas *blockGas) {
	account, err := getContract(txn, addr)
	if err != nil {
		return // no contract slot on this address
	}

	if !account.IsActive() || len(account.Source) == 0 {
		return
	}

	run := ContractRun{Contract: addr.Bytes(), Entry: entry}

	// the deterministic block budget: a drained block skips the run,
	// visibly, so wallets learn to spread heavy calls across blocks
	if gas != nil && gas.remaining == 0 {
		run.Error = "block gas budget exhausted"
		recordRun(txn, tx, run)
		return
	}

	vm, err := NewVM(txn, account, tx, blockTime)
	if err != nil {
		log.Errorf("failed to build the vm for %s: %v", addr, err)
		run.Error = err.Error()
		recordRun(txn, tx, run)
		return
	}
	vm.cs = state.cs // capture the journal flush's pre-images (archive)

	// the calldata method may address a named export
	entry = vm.EntryFor(entry)
	run.Entry = entry

	// clamp this run to whatever the block has left
	if gas != nil && gas.remaining < vmMaxToll {
		vm.LimitToll(gas.remaining)
	}

	err = vm.Run(entry)
	run.GasUsed = vm.GasUsed()
	if gas != nil {
		if run.GasUsed >= gas.remaining {
			gas.remaining = 0
		} else {
			gas.remaining -= run.GasUsed
		}
	}
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
	if err := appendContractRun(txn, tx.GetHash(), tx.Height, run); err != nil {
		log.Errorf("failed to record the receipt of tx %x: %v", tx.GetHash(), err)
	}
}
