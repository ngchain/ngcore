package ngstate

import (
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// rentTx builds a ZERONET transact tx locked to the given height, so the kv
// host's ForkStateRent gate (keyed on vm.caller.{Network,Height}) sees it as
// post-fork when height >= StateRentForkHeight and pre-fork below it.
func rentTx(height uint64) *ngtypes.FullTx {
	return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, height,
		ngtypes.Address{}, nil, big.NewInt(0), nil, nil)
}

// depBytes is DepositPerByte * n as an int64-safe big.Int for the small n the
// tests use.
func depBytes(n int64) *big.Int {
	return new(big.Int).Mul(ngtypes.DepositPerByte, big.NewInt(n))
}

// postForkHeight is a height at/above the state-rent activation.
const postForkHeight = ngtypes.StateRentForkHeight

// kvSetWatKV writes a chosen key/value pair once (both embedded as data), so a
// test controls the exact byte counts that drive the deposit.
func kvSetWatKV(key, val string) string {
	kb := watBytes([]byte(key))
	vb := watBytes([]byte(val))
	return `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "` + kb + `")
  (data (i32.const 64) "` + vb + `")
  (func (export "ng:main")
    (drop (call $set (i32.const 0) (i32.const ` + itoa(len(key)) + `)
                     (i32.const 64) (i32.const ` + itoa(len(val)) + `)))))
`
}

// kvDelWat sets key->val then deletes it in the same run, so the net deposit
// delta over the run must be zero (lock then full refund).
func kvSetThenDelWat(key, val string) string {
	kb := watBytes([]byte(key))
	vb := watBytes([]byte(val))
	return `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (import "kv" "del" (func $del (param i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "` + kb + `")
  (data (i32.const 64) "` + vb + `")
  (func (export "ng:main")
    (drop (call $set (i32.const 0) (i32.const ` + itoa(len(key)) + `)
                     (i32.const 64) (i32.const ` + itoa(len(val)) + `)))
    (drop (call $del (i32.const 0) (i32.const ` + itoa(len(key)) + `)))))
`
}

func itoa(n int) string {
	return big.NewInt(int64(n)).String()
}

// runVM builds a VM for acc at the tx's height and runs main, returning the run
// error (nil on success).
func runVM(t *testing.T, txn *bbolt.Tx, acc *ngtypes.Contract, tx *ngtypes.FullTx) error {
	t.Helper()
	vm, err := NewVM(txn, acc, tx, 1)
	if err != nil {
		t.Fatal(err)
	}
	return vm.Run(VMEntryOnTx)
}

// TestRentLockOnSet: a funded contract's post-fork kv.set locks
// DepositPerByte*(len(key)+len(value)) from its own balance into the escrow.
func TestRentLockOnSet(t *testing.T) {
	db := newTestDB(t)
	addr := testAddr(0xa1)
	const key, val = "key", "val" // 3 + 3 = 6 bytes

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, mustWat(kvSetWatKV(key, val)), nil)
		acc.SetActive(true)
		fund := new(big.Int).Set(ngtypes.NG) // ample balance
		putContract(t, txn, acc, 0)
		if err := setBalance(txn, nil, addr, fund); err != nil {
			return err
		}

		if err := runVM(t, txn, acc, rentTx(postForkHeight)); err != nil {
			t.Fatalf("post-fork kv.set run failed: %v", err)
		}

		want := depBytes(int64(len(key) + len(val)))
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(want) != 0 {
			t.Fatalf("escrow = %s, want %s", got, want)
		}
		expContract := new(big.Int).Sub(fund, want)
		if got := getBalance(txn, addr); got.Cmp(expContract) != 0 {
			t.Fatalf("contract balance = %s, want %s", got, expContract)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRentGrowShrink: growing an entry locks only the DELTA; shrinking refunds
// the delta. Two runs on the same slot with different value sizes.
func TestRentGrowShrink(t *testing.T) {
	db := newTestDB(t)
	addr := testAddr(0xa2)

	err := db.Update(func(txn *bbolt.Tx) error {
		fund := new(big.Int).Set(ngtypes.NG)
		// first: key(3) + "v"(1) = 4 bytes
		acc := ngtypes.NewContract(addr, mustWat(kvSetWatKV("key", "v")), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)
		if err := setBalance(txn, nil, addr, fund); err != nil {
			return err
		}
		if err := runVM(t, txn, acc, rentTx(postForkHeight)); err != nil {
			t.Fatalf("first set run failed: %v", err)
		}
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(depBytes(4)) != 0 {
			t.Fatalf("escrow after set(4) = %s, want %s", got, depBytes(4))
		}

		// grow: same key, value "vvvvv"(5) -> 3+5 = 8 bytes, delta +4. Swap the
		// SOURCE on the existing slot, preserving its Context (the stored entry)
		grow, _ := getContract(txn, addr)
		grow.Source = mustWat(kvSetWatKV("key", "vvvvv"))
		if err := setContract(txn, nil, grow); err != nil {
			return err
		}
		grow, _ = getContract(txn, addr)
		if err := runVM(t, txn, grow, rentTx(postForkHeight)); err != nil {
			t.Fatalf("grow run failed: %v", err)
		}
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(depBytes(8)) != 0 {
			t.Fatalf("escrow after grow to 8 = %s, want %s", got, depBytes(8))
		}

		// shrink: same key, value "vv"(2) -> 3+2 = 5 bytes, delta -3
		shrink, _ := getContract(txn, addr)
		shrink.Source = mustWat(kvSetWatKV("key", "vv"))
		if err := setContract(txn, nil, shrink); err != nil {
			return err
		}
		shrink, _ = getContract(txn, addr)
		if err := runVM(t, txn, shrink, rentTx(postForkHeight)); err != nil {
			t.Fatalf("shrink run failed: %v", err)
		}
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(depBytes(5)) != 0 {
			t.Fatalf("escrow after shrink to 5 = %s, want %s", got, depBytes(5))
		}
		// contract balance mirrors: fund - depBytes(5)
		exp := new(big.Int).Sub(fund, depBytes(5))
		if got := getBalance(txn, addr); got.Cmp(exp) != 0 {
			t.Fatalf("contract balance = %s, want %s", got, exp)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRentDelRefunds: a set+del in one run nets to zero deposit — the del
// refunds the whole bond the set locked.
func TestRentDelRefunds(t *testing.T) {
	db := newTestDB(t)
	addr := testAddr(0xa3)

	err := db.Update(func(txn *bbolt.Tx) error {
		fund := new(big.Int).Set(ngtypes.NG)
		acc := ngtypes.NewContract(addr, mustWat(kvSetThenDelWat("key", "value")), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)
		if err := setBalance(txn, nil, addr, fund); err != nil {
			return err
		}
		if err := runVM(t, txn, acc, rentTx(postForkHeight)); err != nil {
			t.Fatalf("set+del run failed: %v", err)
		}
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Sign() != 0 {
			t.Fatalf("escrow after set+del = %s, want 0", got)
		}
		if got := getBalance(txn, addr); got.Cmp(fund) != 0 {
			t.Fatalf("contract balance = %s, want %s (fully refunded)", got, fund)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRentUnfundedSoftFails: an UNFUNDED contract's post-fork kv.set cannot
// cover the deposit, so the run soft-fails and writes nothing — the base tx
// stands (no consensus break), the escrow stays empty.
func TestRentUnfundedSoftFails(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}
	addr := testAddr(0xa4)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, mustWat(kvSetWatKV("key", "val")), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0) // balance 0 -> cannot pay the deposit

		tx := rentTx(postForkHeight)
		// runContract records the run and never fails the tx; the deposit panic
		// is recovered into a failed run
		ok := state.runContract(txn, addr, tx, VMEntryOnTx, 1, nil)
		if ok {
			t.Fatal("unfunded post-fork kv.set must soft-fail, got ok")
		}

		runs, err := GetTxRuns(txn, tx.GetHash())
		if err != nil {
			return err
		}
		if len(runs) != 1 || runs[0].Ok || runs[0].Error == "" {
			t.Fatalf("runs = %+v, want one recorded failure", runs)
		}

		// nothing written, escrow untouched
		reloaded, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if reloaded.Context.Has("key") {
			t.Fatal("soft-failed run must write no kv")
		}
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Sign() != 0 {
			t.Fatalf("escrow = %s, want 0 (nothing locked)", got)
		}
		if got := getBalance(txn, addr); got.Sign() != 0 {
			t.Fatalf("contract balance = %s, want 0 (untouched)", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRentPreForkUnchanged: BELOW StateRentForkHeight, kv.set/del touch no
// balances and no escrow — byte-for-byte the old behavior (an unfunded
// contract still writes fine).
func TestRentPreForkUnchanged(t *testing.T) {
	db := newTestDB(t)
	addr := testAddr(0xa5)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, mustWat(kvSetWatKV("key", "val")), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0) // unfunded

		// pre-fork height: below the activation
		if err := runVM(t, txn, acc, rentTx(postForkHeight-1)); err != nil {
			t.Fatalf("pre-fork kv.set run failed: %v", err)
		}

		reloaded, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if got := string(reloaded.Context.Get("key")); got != "val" {
			t.Fatalf("pre-fork kv.set not applied, got %q", got)
		}
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Sign() != 0 {
			t.Fatalf("pre-fork escrow = %s, want 0 (no deposit logic)", got)
		}
		if got := getBalance(txn, addr); got.Sign() != 0 {
			t.Fatalf("pre-fork contract balance = %s, want 0 (untouched)", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRentDestroyRefundsAndConservesSupply: destroying a contract refunds its
// whole locked deposit to its address balance and zeroes the escrow. Supply is
// conserved: the contract's address ends with its funding back minus only the
// deploy/tx fees the chain burned — the deposit round-trips exactly.
func TestRentDestroyRefunds(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}
	addr := testAddr(0xa6)
	const key, val = "key", "val" // 6 bytes

	err := db.Update(func(txn *bbolt.Tx) error {
		fund := new(big.Int).Set(ngtypes.NG)
		acc := ngtypes.NewContract(addr, mustWat(kvSetWatKV(key, val)), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)
		if err := setBalance(txn, nil, addr, fund); err != nil {
			return err
		}

		// lock a deposit via a post-fork kv.set
		if err := runVM(t, txn, acc, rentTx(postForkHeight)); err != nil {
			t.Fatalf("set run failed: %v", err)
		}
		locked := depBytes(int64(len(key) + len(val)))
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(locked) != 0 {
			t.Fatalf("escrow after set = %s, want %s", got, locked)
		}

		// reload the slot (now carrying the kv entry) and refund on destroy
		slot, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if err := refundContractDeposit(txn, state.cs, addr, slot.Context); err != nil {
			return err
		}
		if err := delContract(txn, state.cs, addr); err != nil {
			return err
		}

		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Sign() != 0 {
			t.Fatalf("escrow after destroy = %s, want 0", got)
		}
		// supply conserved: the whole deposit is back on the address
		if got := getBalance(txn, addr); got.Cmp(fund) != 0 {
			t.Fatalf("contract balance after destroy = %s, want %s (deposit fully refunded)", got, fund)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
