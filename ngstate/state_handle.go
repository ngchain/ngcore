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
		case ngtypes.TransactTx:
			if err := state.consumeReveal(txn, tx); err != nil {
				return err
			}
			if err := state.runValidate(txn, tx, blockTime, gas); err != nil {
				return err
			}
			if err := state.handleTransaction(txn, tx, blockTime, gas); err != nil {
				return err
			}
		case ngtypes.DeployTx: // deploy / upgrade tx
			if err := state.consumeReveal(txn, tx); err != nil {
				return err
			}
			if err := state.runValidate(txn, tx, blockTime, gas); err != nil {
				return err
			}
			if err := state.handleDeploy(txn, tx, blockTime, gas); err != nil {
				return err
			}
		default:
			return errors.Wrapf(ngtypes.ErrTxTypeInvalid, "unknown tx type %d", tx.Type)
		}
	}

	return nil
}

// consumeReveal spends the commitment an effect tx reveals: it re-derives the
// commitment hash, locates the in-window unrevealed commitment (matched on
// From and Hash, recorded strictly earlier than this reveal), and deletes it.
// A missing match fails the block — CheckBlockTxs/checkReveal must have passed
// first, so this only fires on an inconsistent apply. Genesis (height 0) is
// exempt: its txs bypass every tx check.
func (state *State) consumeReveal(txn *bbolt.Tx, tx *ngtypes.FullTx) error {
	if tx.Height == 0 {
		return nil
	}
	// the authoritative gate at block-apply: an under-entropy salt is rejected
	// here too, so an adversarial block cannot bypass the pool-side check
	if len(tx.Salt) < ngtypes.MinSaltSize {
		return errors.Wrapf(ErrSaltTooShort, "salt is %d bytes, need >= %d", len(tx.Salt), ngtypes.MinSaltSize)
	}

	from, err := tx.From()
	if err != nil {
		return err
	}

	hash := revealHash(tx)
	h, ok := findCommit(txn, from, hash, tx.Height)
	if !ok {
		return errors.Wrapf(ErrTxNotCommitted,
			"no in-window unrevealed commitment for %s revealing at height %d", from, tx.Height)
	}

	// journal the consumption at the reveal's height BEFORE deleting, so a
	// block-undo of this reveal restores the commitment (its recording block
	// may stay canonical below the reorg's fork point)
	if err := journalConsumed(txn, tx.Height, h, hash, from); err != nil {
		return err
	}
	return consumeCommit(txn, h, hash)
}

// runValidate is the native account-abstraction gate. When the tx's From
// address has a LIVE contract exporting `validate`, the protocol runs it to
// authorize the tx ON TOP OF the native signature: the account programs its
// own policy (spend limits, freezes, rate limits, allow-lists, multi-factor)
// and the hook decides via the tx.* context whether to permit this tx.
//
// Unlike the `upgrade` hook (which soft-fails, keeping the tx valid), a
// validate veto is a HARD failure: the tx — and thus the block carrying it —
// is invalid, so a miner must exclude any tx its sender's policy rejects.
// An account with no live `validate` export is unaffected: the native
// signature alone authorizes it, exactly as before. The run draws from the
// SAME deterministic block-gas budget as any other contract call, so every
// replica reaches the identical verdict.
func (state *State) runValidate(txn *bbolt.Tx, tx *ngtypes.FullTx, blockTime uint64, gas *blockGas) error {
	from, err := tx.From()
	if err != nil {
		return err
	}

	account, err := getContract(txn, from)
	if err != nil {
		return nil // no contract slot: the native signature authorizes
	}
	if !account.IsActive() || len(account.Source) == 0 {
		return nil // dormant/empty slot: no policy to enforce
	}
	if !contractHasExport(account.Source, VMEntryOnValidate) {
		return nil // the account programs no policy
	}

	if ok := state.runContract(txn, from, tx, VMEntryOnValidate, blockTime, gas); !ok {
		return errors.Wrapf(ErrTxUnauthorized, "sender %s policy rejected the tx", from)
	}

	return nil
}

func (state *State) handleGenerate(txn *bbolt.Tx, tx *ngtypes.FullTx) (err error) {
	// unsigned generates are uncle rewards: checkBlockGenerates already
	// bound them (recipient + amount) to the block's uncle set, so there is
	// no signer to verify — just mint to the orphaned miner
	if len(tx.Sign) != 0 {
		if err := tx.Verify(keyResolver(txn)); err != nil {
			return err
		}

		if err := registerPubKey(txn, state.cs, tx); err != nil {
			return err
		}
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

// handleDeploy deploys or upgrades the sender's own contract slot from the
// whole compiled module carried in Extra (UUPS lifecycle in one op):
//
//   - EMPTY slot -> deploy: compile, pin the live dependencies, open the
//     slot LIVE, and run the optional `init` hook once (merges the old
//     commit+activate).
//   - LIVE slot -> upgrade: run the current contract's `upgrade` hook; if
//     it traps the code is kept and the failed run is recorded (soft fail,
//     the tx stays valid and the fee is paid). On success the old deps are
//     released, the new deps pinned, the code replaced in place (still
//     live), and `init` re-run as a migration hook.
func (state *State) handleDeploy(txn *bbolt.Tx, tx *ngtypes.FullTx, blockTime uint64, gas *blockGas) (err error) {
	if err := tx.CheckDeploy(keyResolver(txn)); err != nil {
		return err
	}

	from, err := chargeFrom(txn, state.cs, tx, tx.TotalExpenditure())
	if err != nil {
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

	slot, hasSlot := getContract(txn, from)
	isLive := hasSlot == nil && slot.IsActive() && len(slot.Source) != 0

	// EMPTY code on a LIVE slot -> DESTROY: the contract authorizes its own
	// removal through the SAME `upgrade` hook (it sees the empty proposed code
	// via the tx). Refused while other contracts still depend on it.
	if len(newSource) == 0 {
		if !isLive {
			return ErrNothingToDestroy
		}
		if refs := getRefCount(slot); refs > 0 {
			return errors.Wrapf(ErrContractRefdBy, "%d dependent contract(s)", refs)
		}
		if ok := state.runContract(txn, from, tx, VMEntryOnUpgrade, blockTime, gas); !ok {
			return nil // the hook rejected the destroy — soft-fail, slot kept
		}
		if err := releaseContractDeps(txn, state.cs, slot); err != nil {
			return err
		}
		return delContract(txn, state.cs, from)
	}

	// the module must compile before it can go live
	if _, err := LoadContractWasm(newSource); err != nil {
		return err
	}

	// resolve and validate the new module's live dependencies
	newDeps, err := validateDeps(txn, from, newSource)
	if err != nil {
		return err
	}

	if isLive {
		// LIVE slot -> UUPS upgrade: the current code authorizes the swap.
		// A trapped upgrade hook soft-fails: the code is NOT replaced, the
		// failed run is recorded, the tx stays valid (fee already paid)
		if ok := state.runContract(txn, from, tx, VMEntryOnUpgrade, blockTime, gas); !ok {
			return nil
		}

		// release the references the OLD code held on its dependencies
		if err := releaseContractDeps(txn, state.cs, slot); err != nil {
			return err
		}
	} else {
		// EMPTY slot -> deploy: open the namespace
		slot = ngtypes.NewContract(from, nil, nil)
	}

	// pin the references the NEW code holds on its dependencies
	if err := pinDeps(txn, state.cs, newDeps); err != nil {
		return err
	}

	slot.Source = newSource
	slot.SetActive(true)
	if err := setContractDeps(slot, newDeps); err != nil {
		return err
	}
	if err := setContract(txn, state.cs, slot); err != nil {
		return err
	}

	// the optional `init` hook runs once on deploy, and again as a migration
	// hook after an upgrade
	state.runContract(txn, from, tx, VMEntryOnActivate, blockTime, gas)

	return nil
}

// validateDeps resolves the module dependencies imported by source: each
// must be a live contract and not the deployer itself
func validateDeps(txn *bbolt.Tx, from ngtypes.Address, source []byte) ([]ngtypes.Address, error) {
	deps, err := extractContractDeps(source)
	if err != nil {
		return nil, err
	}
	for _, depAddr := range deps {
		if depAddr.Equals(from) {
			return nil, ErrDepSelf
		}

		depAcc, err := getContract(txn, depAddr)
		if err != nil {
			return nil, errors.Wrapf(err, "unknown dependency contract %s", depAddr)
		}
		if !depAcc.IsActive() || len(depAcc.Source) == 0 {
			return nil, errors.Wrapf(ErrDepNotActive, "contract %s", depAddr)
		}
	}

	return deps, nil
}

// pinDeps bumps the refcount of each dependency by one, keeping it alive
// while this contract links against it
func pinDeps(txn *bbolt.Tx, cs *changeset, deps []ngtypes.Address) error {
	for _, depAddr := range deps {
		depAcc, err := getContract(txn, depAddr)
		if err != nil {
			return errors.Wrapf(err, "unknown dependency contract %s", depAddr)
		}

		setRefCount(depAcc, getRefCount(depAcc)+1)
		if err := setContract(txn, cs, depAcc); err != nil {
			return err
		}
	}

	return nil
}

// releaseContractDeps drops the references a contract held on its recorded
// dependencies by one each
func releaseContractDeps(txn *bbolt.Tx, cs *changeset, slot *ngtypes.Contract) error {
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
			if err := setContract(txn, cs, depAcc); err != nil {
				return err
			}
		}
	}

	return nil
}

// runContract executes the entry export of the address's contract, if
// its slot is locked and has one. A contract failure is final for this
// call (its journal is dropped) but NEVER fails the tx itself: every
// node hits the same result, so consensus is kept. It reports whether the
// entry ran to completion, which the deploy path uses to gate a UUPS
// upgrade on the `upgrade` hook succeeding
func (state *State) runContract(txn *bbolt.Tx, addr ngtypes.Address, tx *ngtypes.FullTx, entry string, blockTime uint64, gas *blockGas) bool {
	account, err := getContract(txn, addr)
	if err != nil {
		return false // no contract slot on this address
	}

	if !account.IsActive() || len(account.Source) == 0 {
		return false
	}

	run := ContractRun{Contract: addr.Bytes(), Entry: entry}

	// the deterministic block budget: a drained block skips the run,
	// visibly, so wallets learn to spread heavy calls across blocks
	if gas != nil && gas.remaining == 0 {
		run.Error = "block gas budget exhausted"
		recordRun(txn, tx, run)
		return false
	}

	vm, err := NewVM(txn, account, tx, blockTime)
	if err != nil {
		log.Errorf("failed to build the vm for %s: %v", addr, err)
		run.Error = err.Error()
		recordRun(txn, tx, run)
		return false
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
	run.Trace = vm.Trace() // kept on failure too, to show where it reverted
	if gas != nil {
		if run.GasUsed >= gas.remaining {
			gas.remaining = 0
		} else {
			gas.remaining -= run.GasUsed
		}
	}
	if err != nil {
		if IsExportMissing(err) && entry != VMEntryOnTx {
			return false // optional entry (e.g. init) is absent — no run to record
		}

		log.Errorf("contract call %s on %s failed: %v", entry, addr, err)
		run.Error = err.Error()
		recordRun(txn, tx, run)
		return false
	}

	run.Ok = true
	run.Events = vm.Events()
	recordRun(txn, tx, run)

	return true
}

// recordRun appends the run to the tx's local receipt; receipt failures
// must never fail consensus, so they only log
func recordRun(txn *bbolt.Tx, tx *ngtypes.FullTx, run ContractRun) {
	if err := appendContractRun(txn, tx.GetHash(), tx.Height, run); err != nil {
		log.Errorf("failed to record the receipt of tx %x: %v", tx.GetHash(), err)
	}
}
