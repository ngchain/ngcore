package ngstate

import (
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// applyAt simulates one block apply at the given height with archive
// recording on, running fn against the write txn under a fresh recorder —
// exactly what Upgrade does around HandleTxs
func applyAt(t *testing.T, state *State, height uint64, fn func(txn *bbolt.Tx)) {
	t.Helper()
	if err := state.Update(func(txn *bbolt.Tx) error {
		state.cs = newChangeset(height)
		defer func() { state.cs = nil }()
		fn(txn)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestArchiveBalanceHistory pins the core changeset+index read path: the
// pre-image recorded at the first change AFTER a height is the value that
// held AT that height, and the current plain state answers once an
// address stops changing
func TestArchiveBalanceHistory(t *testing.T) {
	db := newTestDB(t)
	state := newTestState(t, db)
	state.Archive = true

	addr := testAddr(0xA1)

	// height 1: 0 -> 100, height 2: 100 -> 200, height 3: 200 -> 300
	applyAt(t, state, 1, func(txn *bbolt.Tx) { _ = setBalance(txn, state.cs, addr, big.NewInt(100)) })
	applyAt(t, state, 2, func(txn *bbolt.Tx) { _ = setBalance(txn, state.cs, addr, big.NewInt(200)) })
	applyAt(t, state, 3, func(txn *bbolt.Tx) { _ = setBalance(txn, state.cs, addr, big.NewInt(300)) })

	want := map[uint64]int64{0: 0, 1: 100, 2: 200, 3: 300, 4: 300}
	_ = state.View(func(txn *bbolt.Tx) error {
		for h, w := range want {
			if got := balanceAtHeight(txn, addr, h); got.Int64() != w {
				t.Errorf("balanceAtHeight(%d) = %s, want %d", h, got, w)
			}
		}
		// an address that never appears reads 0 at any height
		if got := balanceAtHeight(txn, testAddr(0xFF), 2); got.Sign() != 0 {
			t.Errorf("unknown addr balance = %s, want 0", got)
		}
		return nil
	})
}

// TestArchiveUnwind pins the reorg primitive: reverting heights from the
// tip restores each address to its pre-image and drops those heights'
// changeset + index entries, leaving the state as if only the surviving
// heights were ever applied
func TestArchiveUnwind(t *testing.T) {
	db := newTestDB(t)
	state := newTestState(t, db)
	state.Archive = true

	addr := testAddr(0xD4)
	deployed := testAddr(0xD5)

	applyAt(t, state, 1, func(txn *bbolt.Tx) { _ = setBalance(txn, state.cs, addr, big.NewInt(100)) })
	applyAt(t, state, 2, func(txn *bbolt.Tx) { _ = setBalance(txn, state.cs, addr, big.NewInt(200)) })
	applyAt(t, state, 3, func(txn *bbolt.Tx) {
		_ = setBalance(txn, state.cs, addr, big.NewInt(300))
		_ = setContract(txn, state.cs, ngtypes.NewContract(deployed, nil, nil))
	})

	// unwind heights 3 and 2: back to the end-of-height-1 state
	if err := state.Update(func(txn *bbolt.Tx) error {
		unwindHeightTxn(txn, 3)
		unwindHeightTxn(txn, 2)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	_ = state.View(func(txn *bbolt.Tx) error {
		if got := getBalance(txn, addr); got.Int64() != 100 {
			t.Errorf("after unwind balance = %s, want 100", got)
		}
		// the height-3 deploy is gone (pre-image was absent)
		if _, err := getContract(txn, deployed); err == nil {
			t.Error("unwound deploy must no longer have a contract slot")
		}
		// changeset + index entries for the unwound heights are cleared
		if _, ok := firstChangeHeightAfter(txn, storage.BalHistBucketName, addr, 1); ok {
			t.Error("history above the surviving tip must be cleared")
		}
		return nil
	})
}

// TestArchiveContractHistory pins the present/absent tombstone: a slot
// deployed at height 5 does not exist at height 4, and exists from 5 on
func TestArchiveContractHistory(t *testing.T) {
	db := newTestDB(t)
	state := newTestState(t, db)
	state.Archive = true

	addr := testAddr(0xB2)

	applyAt(t, state, 5, func(txn *bbolt.Tx) {
		_ = setContract(txn, state.cs, ngtypes.NewContract(addr, nil, nil))
	})

	_ = state.View(func(txn *bbolt.Tx) error {
		if _, ok, _ := contractAtHeight(txn, addr, 4); ok {
			t.Error("contract must not exist at height 4 (deployed at 5)")
		}
		if _, ok, _ := contractAtHeight(txn, addr, 5); !ok {
			t.Error("contract must exist at height 5")
		}
		if _, ok, _ := contractAtHeight(txn, addr, 9); !ok {
			t.Error("contract must exist at height 9 (unchanged since 5)")
		}
		return nil
	})
}

// TestArchiveDisabledRejects pins the guard: a non-archive node refuses
// historical reads instead of returning a wrong (current) answer
func TestArchiveDisabledRejects(t *testing.T) {
	db := newTestDB(t)
	state := newTestState(t, db)
	// Archive stays false

	if _, err := state.GetBalanceByAddressAt(testAddr(0x01), 1); err != ErrArchiveDisabled {
		t.Fatalf("GetBalanceByAddressAt err = %v, want ErrArchiveDisabled", err)
	}
	if _, err := state.GetContractAt(testAddr(0x01), 1); err != ErrArchiveDisabled {
		t.Fatalf("GetContractAt err = %v, want ErrArchiveDisabled", err)
	}
}

// TestArchiveOffCapturesNothing pins that with archive off, the write
// path records no changesets (no storage cost, no behavior change)
func TestArchiveOffCapturesNothing(t *testing.T) {
	db := newTestDB(t)
	state := newTestState(t, db)
	// Archive off: cs stays nil even across a simulated apply

	if err := state.Update(func(txn *bbolt.Tx) error {
		return setBalance(txn, state.cs, testAddr(0xC3), big.NewInt(7))
	}); err != nil {
		t.Fatal(err)
	}

	_ = state.View(func(txn *bbolt.Tx) error {
		if _, ok := firstChangeHeightAfter(txn, storage.BalHistBucketName, testAddr(0xC3), 0); ok {
			t.Error("archive-off write must not populate the history index")
		}
		return nil
	})
}
