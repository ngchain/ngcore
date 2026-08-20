package blockchain_test

import (
	"errors"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestReorgTakesUnwindPath proves a reorg on an archive node resolves via
// the changeset unwind (O(reorg depth)), NOT the replay-from-genesis
// fallback — so the correctness that TestReorgToHeavierBranch checks is
// actually the unwind path's, and the fallback only covers non-archive
func TestReorgTakesUnwindPath(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, miner)
	a2 := mineBlock(t, b1, miner)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(a2); err != nil {
		t.Fatal(err)
	}

	errRollback := errors.New("probe rollback") // never persist the probe

	// archive is on by default: unwinding to the fork height must succeed
	var ok bool
	_ = chain.State.Update(func(txn *bbolt.Tx) error {
		var err error
		ok, err = chain.State.UnwindToTxn(txn, 1)
		if err != nil {
			t.Fatal(err)
		}
		return errRollback
	})
	if !ok {
		t.Fatal("archive node must take the unwind path for a fork at height 1")
	}

	// with archive off the node must decline, forcing the replay fallback
	chain.State.Archive = false
	var off bool
	_ = chain.State.Update(func(txn *bbolt.Tx) error {
		off, _ = chain.State.UnwindToTxn(txn, 1)
		return errRollback
	})
	if off {
		t.Fatal("a non-archive node must not unwind (must replay instead)")
	}
}

// TestArchiveBackfill simulates an in-place upgrade of a pre-archive db:
// blocks were applied with archive off (no changesets), then the node
// restarts in archive mode. BackfillArchive must rebuild the history so
// past-height reads work, and be a no-op on the second call
func TestArchiveBackfill(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	// pre-archive: apply three blocks WITHOUT capturing changesets
	chain.State.Archive = false
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	prev := genesis
	for i := 0; i < 3; i++ {
		b := mineBlock(t, prev, miner)
		if err := chain.ApplyBlock(b); err != nil {
			t.Fatal(err)
		}
		prev = b
	}

	// without the history, a past-height read silently falls back to the
	// CURRENT balance (the gap backfill closes): height 1 wrongly reads the
	// three-block total
	chain.State.Archive = true
	tipTotal := new(big.Int).Add(
		new(big.Int).Add(ngtypes.GetBlockReward(1), ngtypes.GetBlockReward(2)),
		ngtypes.GetBlockReward(3))
	if got, err := chain.State.GetBalanceByAddressAt(ngtypes.NewAddress(miner), 1); err != nil {
		t.Fatal(err)
	} else if got.Cmp(tipTotal) != 0 {
		t.Fatalf("pre-backfill balance@1 = %s, want the (wrong) tip total %s", got, tipTotal)
	}

	// backfill rebuilds the changeset history from the block store
	did, err := chain.State.BackfillArchive()
	if err != nil {
		t.Fatal(err)
	}
	if !did {
		t.Fatal("backfill must run when changesets are absent")
	}

	// now the height-1 balance is the single block reward, not the tip total
	got, err := chain.State.GetBalanceByAddressAt(ngtypes.NewAddress(miner), 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := ngtypes.GetBlockReward(1); got.Cmp(want) != 0 {
		t.Fatalf("balance@1 after backfill = %s, want %s", got, want)
	}

	// idempotent: a second call finds full coverage and does nothing
	if did, err := chain.State.BackfillArchive(); err != nil || did {
		t.Fatalf("second backfill = (%v, %v), want (false, nil)", did, err)
	}
}
