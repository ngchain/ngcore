package ngstate

import (
	"math/big"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// Storage-deposit (state-rent) moves, gated by ForkStateRent at the call sites.
//
// A deposit proportional to a contract's stored bytes is LOCKED from the
// contract's own native balance into the reserved escrow (StorageDepositEscrow)
// when its kv grows, and REFUNDED when its kv shrinks / a key is deleted / the
// contract is destroyed. Both directions go through the SAME journaled balance
// path coin.transfer uses (vm.journal.transfer), so a failed contract call
// discards them together with every other journaled write — nothing is final
// until the whole call succeeds, and a reorg unwind restores balances exactly
// as it does for an ordinary transfer.

// ErrStorageDeposit signals the executing contract cannot cover the storage
// deposit its kv write would lock. The kv host turns it into a panic, which the
// wasman Recover soft-fails: the run is recorded failed, its journal (this
// deposit move included) is dropped, and consensus is untouched.
var ErrStorageDeposit = errors.New("insufficient balance for storage deposit")

// contextKeyRent is the reserved context key holding the running total a
// contract has ACTUALLY locked into the escrow, as big.Int bytes. It is the
// primary bound on every refund: a contract can never be paid back more than it
// itself locked, so it cannot siphon another contract's bond — even across the
// ForkStateRent boundary, where entries written PRE-fork locked nothing and so
// contributed 0 to this total. It is written through the SAME journaled context
// the kv host mutates (protocol-side, bypassing the reserved-key trap), so a
// trapped run or a reorg unwind restores it in lockstep with the balances.
//
// It is "_"-prefixed and short, hence reserved: contracts cannot touch it (the
// kv host skips reserved keys), and it is never itself charged a deposit — the
// kv.set/del paths that lock/refund never fire for reserved keys.
const contextKeyRent = "_rent"

// lockedDeposit reads a contract's running locked-deposit total from its
// context. Absent (never locked, e.g. a pre-fork-only contract) reads as zero.
func lockedDeposit(ctx *ngtypes.ContractContext) *big.Int {
	raw := ctx.Get(contextKeyRent)
	if len(raw) == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(raw)
}

// setLockedDeposit writes a contract's running locked-deposit total back
// through the (journaled) context. A zero total deletes the key, keeping a
// contract that never locked anything byte-identical to the pre-fork encoding.
func setLockedDeposit(ctx *ngtypes.ContractContext, amount *big.Int) {
	if amount.Sign() <= 0 {
		ctx.Del(contextKeyRent)
		return
	}
	ctx.Set(contextKeyRent, amount.Bytes())
}

// depositFor returns DepositPerByte * bytes, the bond owed for `bytes` of
// stored kv (key+value length). bytes is always non-negative at the call sites.
func depositFor(bytes int) *big.Int {
	return new(big.Int).Mul(ngtypes.DepositPerByte, big.NewInt(int64(bytes)))
}

// lockDeposit moves `amount` from the executing contract to the storage-deposit
// escrow through the journal, and bumps the contract's running locked-deposit
// total (_rent) by that same amount. If the contract's balance cannot cover it,
// it returns ErrStorageDeposit so the caller panics and the whole call
// soft-fails (writing nothing — the _rent bump included, journaled together). A
// non-positive amount is a no-op.
func (vm *VM) lockDeposit(amount *big.Int) error {
	if amount.Sign() <= 0 {
		return nil
	}

	from := vm.currentAddress()
	if err := vm.journal.transfer(vm.txn, from, ngtypes.StorageDepositEscrow, amount); err != nil {
		// balance-insufficient (or any transfer refusal) becomes the deposit error
		return errors.Wrap(ErrStorageDeposit, err.Error())
	}

	// grow the actually-locked total on the SAME journaled context the kv host
	// mutates, so it rolls back with the transfer on a trap / reorg unwind.
	ctx := vmContext(vm)
	setLockedDeposit(ctx, new(big.Int).Add(lockedDeposit(ctx), amount))
	return nil
}

// refundDeposit moves a refund from the escrow back to the executing contract
// through the journal, bounded by what THIS contract actually locked. The pay
// is min(amount, _rent, escrowBalance):
//
//   - _rent is the primary bound: a contract can never be refunded more than it
//     itself locked, so it cannot drain another contract's bond. An entry
//     written PRE-fork contributed 0 to _rent, so deleting/shrinking it here
//     refunds nothing — closing the cross-fork theft vector.
//   - the escrow-balance cap stays as defense-in-depth (it should never bind
//     once _rent bounds the payout), failing SAFE rather than minting.
//
// On payout _rent shrinks by the same amount, kept on the journaled context so
// it unwinds with the balances. A non-positive amount is a no-op.
func (vm *VM) refundDeposit(amount *big.Int) {
	if amount.Sign() <= 0 {
		return
	}

	ctx := vmContext(vm)
	locked := lockedDeposit(ctx)

	pay := amount
	if locked.Cmp(pay) < 0 {
		// only what this contract actually locked can come back — pre-fork bytes
		// locked nothing, so this floors the refund at 0 for them.
		pay = locked
	}

	escrowBal := vm.journal.balanceOf(vm.txn, ngtypes.StorageDepositEscrow)
	if escrowBal.Cmp(pay) < 0 {
		vm.logger.Errorf("storage-deposit escrow underflow: refund %s exceeds escrow %s; capping",
			pay, escrowBal)
		pay = escrowBal
	}
	if pay.Sign() <= 0 {
		return
	}

	if err := vm.journal.transfer(vm.txn, ngtypes.StorageDepositEscrow, vm.currentAddress(), pay); err != nil {
		// unreachable: pay <= escrow balance, so the transfer cannot underflow
		vm.logger.Errorf("storage-deposit refund failed: %v", err)
		return
	}

	setLockedDeposit(ctx, new(big.Int).Sub(locked, pay))
}

// refundContractDeposit moves a contract's ENTIRE actually-locked storage
// deposit — its stored _rent, NOT a byte-sum — from the escrow back to its own
// balance, then it vanishes with the slot. This is a PROTOCOL balance move (not
// inside a VM run), run through the state's changeset-aware setBalance path (the
// same one handleDeploy uses), so a reorg unwind restores both balances. It
// refunds exactly _rent (a contract that only ever wrote pre-fork bytes locked
// nothing, so _rent is absent/zero and it gets 0 — it cannot drain the escrow),
// capped at the escrow balance to fail SAFE rather than mint if the two ever
// diverged. Called on destroy, POST-ForkStateRent only.
func refundContractDeposit(txn *bbolt.Tx, cs *changeset, addr ngtypes.Address, ctx *ngtypes.ContractContext) error {
	owed := lockedDeposit(ctx)
	if owed.Sign() <= 0 {
		return nil
	}

	escrowBal := getBalance(txn, ngtypes.StorageDepositEscrow)
	pay := owed
	if escrowBal.Cmp(owed) < 0 {
		log.Errorf("storage-deposit escrow underflow on destroy of %s: owed %s exceeds escrow %s; capping",
			addr, owed, escrowBal)
		pay = escrowBal
	}
	if pay.Sign() <= 0 {
		return nil
	}

	if err := setBalance(txn, cs, ngtypes.StorageDepositEscrow, new(big.Int).Sub(escrowBal, pay)); err != nil {
		return err
	}
	return setBalance(txn, cs, addr, new(big.Int).Add(getBalance(txn, addr), pay))
}
