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

// depositFor returns DepositPerByte * bytes, the bond owed for `bytes` of
// stored kv (key+value length). bytes is always non-negative at the call sites.
func depositFor(bytes int) *big.Int {
	return new(big.Int).Mul(ngtypes.DepositPerByte, big.NewInt(int64(bytes)))
}

// lockDeposit moves `amount` from the executing contract to the storage-deposit
// escrow through the journal. If the contract's balance cannot cover it, it
// returns ErrStorageDeposit so the caller panics and the whole call soft-fails
// (writing nothing). A non-positive amount is a no-op.
func (vm *VM) lockDeposit(amount *big.Int) error {
	if amount.Sign() <= 0 {
		return nil
	}

	from := vm.currentAddress()
	if err := vm.journal.transfer(vm.txn, from, ngtypes.StorageDepositEscrow, amount); err != nil {
		// balance-insufficient (or any transfer refusal) becomes the deposit error
		return errors.Wrap(ErrStorageDeposit, err.Error())
	}
	return nil
}

// refundDeposit moves up to `amount` from the escrow back to the executing
// contract through the journal. It refunds min(amount, escrowBalance): a
// balanced lock/refund can never underflow the escrow, but capping fails SAFE
// (never minting) if they ever diverged, logging the discrepancy instead of
// panicking. A non-positive amount is a no-op.
func (vm *VM) refundDeposit(amount *big.Int) {
	if amount.Sign() <= 0 {
		return
	}

	escrowBal := vm.journal.balanceOf(vm.txn, ngtypes.StorageDepositEscrow)
	pay := amount
	if escrowBal.Cmp(amount) < 0 {
		vm.logger.Errorf("storage-deposit escrow underflow: refund %s exceeds escrow %s; capping",
			amount, escrowBal)
		pay = escrowBal
	}
	if pay.Sign() <= 0 {
		return
	}

	if err := vm.journal.transfer(vm.txn, ngtypes.StorageDepositEscrow, vm.currentAddress(), pay); err != nil {
		// unreachable: pay <= escrow balance, so the transfer cannot underflow
		vm.logger.Errorf("storage-deposit refund failed: %v", err)
	}
}

// contextDepositTotal is the whole bond a contract's Context owes:
// DepositPerByte * Σ(len(k)+len(v)) over its NON-reserved entries (reserved
// "_"-prefixed keys were never charged, so they are skipped). It recomputes the
// deposit purely from the bytes on chain — no running total is stored.
func contextDepositTotal(ctx *ngtypes.ContractContext) *big.Int {
	bytes := 0
	for i, k := range ctx.Keys {
		if isReservedKey(k) {
			continue
		}
		bytes += len(k)
		if i < len(ctx.Values) {
			bytes += len(ctx.Values[i])
		}
	}
	return depositFor(bytes)
}

// refundContractDeposit moves a contract's ENTIRE locked storage deposit from
// the escrow back to its own balance. This is a PROTOCOL balance move (not
// inside a VM run), run through the state's changeset-aware setBalance path (the
// same one handleDeploy uses), so a reorg unwind restores both balances. It
// refunds min(owed, escrowBalance) to fail SAFE rather than mint if the two ever
// diverged, logging the discrepancy. Called on destroy, POST-ForkStateRent only.
func refundContractDeposit(txn *bbolt.Tx, cs *changeset, addr ngtypes.Address, ctx *ngtypes.ContractContext) error {
	owed := contextDepositTotal(ctx)
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
