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

// kvDelWat deletes a single key (embedded as data), so a test can run a
// standalone delete at a chosen height — independent of the set that created it.
func kvDelWat(key string) string {
	kb := watBytes([]byte(key))
	return `
(module
  (import "kv" "del" (func $del (param i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "` + kb + `")
  (func (export "ng:main")
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

// runSource swaps the SOURCE on an existing slot (preserving its Context) and
// runs main at the tx's height, so a test can drive several ops — potentially
// straddling the fork — against one persistent kv entry.
func runSource(t *testing.T, txn *bbolt.Tx, addr ngtypes.Address, source []byte, tx *ngtypes.FullTx) {
	t.Helper()
	slot, err := getContract(txn, addr)
	if err != nil {
		t.Fatal(err)
	}
	slot.Source = source
	if err := setContract(txn, nil, slot); err != nil {
		t.Fatal(err)
	}
	slot, _ = getContract(txn, addr)
	if err := runVM(t, txn, slot, tx); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

// TestRentCrossForkDelRefundsZero pins the fix: an entry WRITTEN pre-fork locked
// nothing (_rent stays 0), so DELETING it post-fork must refund nothing — the
// contract's balance and the escrow are both untouched by the del. Without the
// _rent bound, depositFor(freed) would pay a refund the contract never funded,
// minting from (and draining) the escrow. A fully post-fork entry, by contrast,
// refunds in full.
func TestRentCrossForkDelRefundsZero(t *testing.T) {
	db := newTestDB(t)
	const key, val = "key", "val" // 6 bytes

	err := db.Update(func(txn *bbolt.Tx) error {
		// --- pre-fork write, post-fork delete: refund must be ZERO ---
		pre := testAddr(0xb1)
		fund := new(big.Int).Set(ngtypes.NG)
		acc := ngtypes.NewContract(pre, mustWat(kvSetWatKV(key, val)), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)
		if err := setBalance(txn, nil, pre, fund); err != nil {
			return err
		}

		// write the entry BELOW the fork: no deposit locked, _rent absent
		runSource(t, txn, pre, mustWat(kvSetWatKV(key, val)), rentTx(postForkHeight-1))
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Sign() != 0 {
			t.Fatalf("pre-fork set escrow = %s, want 0 (nothing locked)", got)
		}
		if got := getBalance(txn, pre); got.Cmp(fund) != 0 {
			t.Fatalf("pre-fork set balance = %s, want %s (untouched)", got, fund)
		}

		// delete the SAME entry ABOVE the fork: _rent is 0, so refund is 0
		runSource(t, txn, pre, mustWat(kvDelWat(key)), rentTx(postForkHeight))
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Sign() != 0 {
			t.Fatalf("post-fork del of pre-fork entry: escrow = %s, want 0 (no phantom refund)", got)
		}
		if got := getBalance(txn, pre); got.Cmp(fund) != 0 {
			t.Fatalf("post-fork del of pre-fork entry: balance = %s, want %s (no phantom refund)", got, fund)
		}

		// --- fully post-fork: set locks, del refunds in full ---
		post := testAddr(0xb2)
		acc2 := ngtypes.NewContract(post, mustWat(kvSetWatKV(key, val)), nil)
		acc2.SetActive(true)
		putContract(t, txn, acc2, 0)
		if err := setBalance(txn, nil, post, fund); err != nil {
			return err
		}
		runSource(t, txn, post, mustWat(kvSetWatKV(key, val)), rentTx(postForkHeight))
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(depBytes(6)) != 0 {
			t.Fatalf("post-fork set escrow = %s, want %s", got, depBytes(6))
		}
		runSource(t, txn, post, mustWat(kvDelWat(key)), rentTx(postForkHeight))
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Sign() != 0 {
			t.Fatalf("post-fork del escrow = %s, want 0 (fully refunded)", got)
		}
		if got := getBalance(txn, post); got.Cmp(fund) != 0 {
			t.Fatalf("post-fork del balance = %s, want %s (fully refunded)", got, fund)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRentNoCrossContractDrain pins the theft vector closed: contract A locks a
// real deposit post-fork (escrow > 0). Contract B, which only ever wrote
// pre-fork data (locking nothing), tries to del AND destroy post-fork. B must
// receive ZERO both ways, and A's bond in the escrow must be untouched — A can
// still be refunded its full amount on its own destroy.
func TestRentNoCrossContractDrain(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}
	const key, val = "key", "val" // 6 bytes

	err := db.Update(func(txn *bbolt.Tx) error {
		fund := new(big.Int).Set(ngtypes.NG)

		// A locks a real deposit post-fork
		a := testAddr(0xc1)
		accA := ngtypes.NewContract(a, mustWat(kvSetWatKV(key, val)), nil)
		accA.SetActive(true)
		putContract(t, txn, accA, 0)
		if err := setBalance(txn, nil, a, fund); err != nil {
			return err
		}
		runSource(t, txn, a, mustWat(kvSetWatKV(key, val)), rentTx(postForkHeight))
		lockedA := depBytes(6)
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(lockedA) != 0 {
			t.Fatalf("escrow after A locks = %s, want %s", got, lockedA)
		}

		// B only ever wrote PRE-fork data — it locked nothing
		b := testAddr(0xc2)
		accB := ngtypes.NewContract(b, mustWat(kvSetWatKV(key, val)), nil)
		accB.SetActive(true)
		putContract(t, txn, accB, 0)
		bFund := new(big.Int).Set(ngtypes.NG)
		if err := setBalance(txn, nil, b, bFund); err != nil {
			return err
		}
		runSource(t, txn, b, mustWat(kvSetWatKV(key, val)), rentTx(postForkHeight-1))

		// B deletes its pre-fork entry post-fork: gets 0, A's escrow untouched
		runSource(t, txn, b, mustWat(kvDelWat(key)), rentTx(postForkHeight))
		if got := getBalance(txn, b); got.Cmp(bFund) != 0 {
			t.Fatalf("B balance after cross-fork del = %s, want %s (no drain)", got, bFund)
		}
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(lockedA) != 0 {
			t.Fatalf("escrow after B's del = %s, want %s (A's bond intact)", got, lockedA)
		}

		// B re-writes the pre-fork entry (still _rent 0) and destroys post-fork:
		// its stored _rent is absent, so destroy refunds 0 — escrow still holds A.
		runSource(t, txn, b, mustWat(kvSetWatKV(key, val)), rentTx(postForkHeight-1))
		slotB, err := getContract(txn, b)
		if err != nil {
			return err
		}
		if err := refundContractDeposit(txn, state.cs, b, slotB.Context); err != nil {
			return err
		}
		if err := delContract(txn, state.cs, b); err != nil {
			return err
		}
		if got := getBalance(txn, b); got.Cmp(bFund) != 0 {
			t.Fatalf("B balance after destroy = %s, want %s (no drain on destroy)", got, bFund)
		}
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(lockedA) != 0 {
			t.Fatalf("escrow after B destroy = %s, want %s (A's bond intact)", got, lockedA)
		}

		// A can still be refunded its FULL bond on its own destroy
		slotA, err := getContract(txn, a)
		if err != nil {
			return err
		}
		if err := refundContractDeposit(txn, state.cs, a, slotA.Context); err != nil {
			return err
		}
		if err := delContract(txn, state.cs, a); err != nil {
			return err
		}
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Sign() != 0 {
			t.Fatalf("escrow after A destroy = %s, want 0 (A refunded in full)", got)
		}
		if got := getBalance(txn, a); got.Cmp(fund) != 0 {
			t.Fatalf("A balance after destroy = %s, want %s (full refund)", got, fund)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
