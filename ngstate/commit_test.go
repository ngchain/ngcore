package ngstate

import (
	"errors"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// revealAt builds a signed, salted transact reveal from priv locked on the
// given height.
func revealAt(t *testing.T, priv *ngtypes.PrivateKey, height uint64) *ngtypes.FullTx {
	t.Helper()

	tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, height,
		testAddr(0x01), big.NewInt(1), big.NewInt(1), nil, nil)
	tx.Salt = effectSalt
	if err := tx.Signature(priv); err != nil {
		t.Fatal(err)
	}
	return tx
}

// TestRevealWithoutCommitmentRejected: an effect tx that reveals nothing (no
// commitment on chain) is rejected as a non-reveal.
func TestRevealWithoutCommitmentRejected(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(100)); err != nil {
			return err
		}

		// no commitment recorded: the reveal is refused
		reveal := revealAt(t, priv, 5)
		if err := CheckTx(txn, reveal); !errors.Is(err, ErrTxNotCommitted) {
			t.Fatalf("uncommitted reveal: got %v, want ErrTxNotCommitted", err)
		}

		// an effect tx with an EMPTY salt is refused too
		noSalt := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 5,
			testAddr(0x01), big.NewInt(1), big.NewInt(1), nil, nil)
		if err := noSalt.Signature(priv); err != nil {
			return err
		}
		if err := CheckTx(txn, noSalt); !errors.Is(err, ErrTxNotCommitted) {
			t.Fatalf("salt-less reveal: got %v, want ErrTxNotCommitted", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRevealWindowRetryAcrossHeights: because the commitment binds the reveal's
// content but NOT its target height, ONE commitment is revealable at any height
// inside the window — so a miner censoring the first candidate block cannot pin
// the reveal. A height outside the window is refused.
func TestRevealWindowRetryAcrossHeights(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	from := ngtypes.NewAddress(priv)

	mkReveal := func(height uint64) *ngtypes.FullTx {
		tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, height,
			testAddr(0x02), big.NewInt(1), big.NewInt(1), nil, nil)
		tx.Salt = effectSalt
		if err := tx.Signature(priv); err != nil {
			t.Fatal(err)
		}
		return tx
	}

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, from, big.NewInt(1000)); err != nil {
			return err
		}
		// the height-independent commitment (built from a reveal at any height)
		c := revealHash(mkReveal(6))
		if err := putCommit(txn, 5, c, from); err != nil {
			return err
		}

		// the SAME commitment@5 (window 3) is revealable at 6, 7 and 8
		for _, rh := range []uint64{6, 7, 8} {
			if _, ok := findCommit(txn, from, revealHash(mkReveal(rh)), rh); !ok {
				t.Fatalf("commitment@5 must stay revealable at height %d (window retry)", rh)
			}
		}
		// but not at 9: commitment@5 falls outside [9-3, 9)
		if _, ok := findCommit(txn, from, revealHash(mkReveal(9)), 9); ok {
			t.Fatal("commitment@5 must be out of window at height 9")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCommitRestoredOnRevealRevert: reverting a reveal block must restore the
// commitment that reveal consumed, even though the commit was recorded at an
// earlier (still-canonical) height. Without the consumption journal a reorged
// node would reject a reveal a fresh-synced node accepts.
func TestCommitRestoredOnRevealRevert(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	from := ngtypes.NewAddress(priv)
	hash := make([]byte, ngtypes.HashSize)
	hash[0], hash[31] = 0xab, 0xcd

	err := db.Update(func(txn *bbolt.Tx) error {
		// commitment recorded at height 5
		if err := putCommit(txn, 5, hash, from); err != nil {
			return err
		}
		// a reveal at height 6 spends it (journaled, then deleted)
		if err := journalConsumed(txn, 6, 5, hash, from); err != nil {
			return err
		}
		if err := consumeCommit(txn, 5, hash); err != nil {
			return err
		}
		if _, ok := findCommit(txn, from, hash, 7); ok {
			t.Fatal("commitment should be consumed after the reveal")
		}

		// unwind the reveal block @6: the commitment @5 must reappear
		restoreConsumedAtHeight(txn, 6)
		if _, ok := findCommit(txn, from, hash, 7); !ok {
			t.Fatal("consumed commitment was not restored on reveal-block revert")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRevealShortSaltRejected: a reveal whose salt is below MinSaltSize is
// refused — a guessable-content commitment with a short salt is grindable, so
// the reveal must never be admissible.
func TestRevealShortSaltRejected(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(100)); err != nil {
			return err
		}

		short := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 5,
			testAddr(0x01), big.NewInt(1), big.NewInt(1), nil, nil)
		short.Salt = make([]byte, ngtypes.MinSaltSize-1) // one byte short
		if err := short.Signature(priv); err != nil {
			return err
		}
		if err := CheckTx(txn, short); !errors.Is(err, ErrSaltTooShort) {
			t.Fatalf("short-salt reveal: got %v, want ErrSaltTooShort", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRevealSameBlockRejected: a commitment recorded at the SAME height as its
// reveal is not a valid earlier commitment — the anti-same-block-reaction rule.
func TestRevealSameBlockRejected(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(100)); err != nil {
			return err
		}

		reveal := revealAt(t, priv, 5)
		// record the commitment at the reveal's OWN height (5): strictly-earlier
		// lookup (h < revealHeight) must not find it
		if err := putCommit(txn, 5, revealHash(reveal), addr); err != nil {
			return err
		}
		if err := CheckTx(txn, reveal); !errors.Is(err, ErrTxNotCommitted) {
			t.Fatalf("same-block reveal: got %v, want ErrTxNotCommitted", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRevealAfterWindowRejected: a commitment older than CommitWindow is out of
// range for the reveal and is refused.
func TestRevealAfterWindowRejected(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(100)); err != nil {
			return err
		}

		// reveal at height 20; the earliest in-window commit height is
		// 20-CommitWindow. A commit one below that is expired
		revealHeight := uint64(20)
		reveal := revealAt(t, priv, revealHeight)
		tooOld := revealHeight - ngtypes.CommitWindow - 1
		if err := putCommit(txn, tooOld, revealHash(reveal), addr); err != nil {
			return err
		}
		if err := CheckTx(txn, reveal); !errors.Is(err, ErrTxNotCommitted) {
			t.Fatalf("expired-commit reveal: got %v, want ErrTxNotCommitted", err)
		}

		// the very edge of the window (revealHeight-CommitWindow) IS admissible
		if err := putCommit(txn, revealHeight-ngtypes.CommitWindow, revealHash(reveal), addr); err != nil {
			return err
		}
		if err := CheckTx(txn, reveal); err != nil {
			t.Fatalf("edge-of-window reveal refused: %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCommitInNRevealInN1: the happy path. A commitment applied via a block at
// height N charges its committer's fee and lands in the store; the reveal in
// block N+1 executes and consumes the commitment.
func TestCommitInNRevealInN1(t *testing.T) {
	db := newTestDB(t)
	state := newTestState(t, db)

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)
	minerPriv, _ := ngtypes.GenerateKey()
	minerAddr := ngtypes.NewAddress(minerPriv)

	// the reveal transfers to this recipient
	dest := testAddr(0x01)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(100)); err != nil {
			return err
		}

		// the reveal lands at height 2; commit it via a block at height 1
		reveal := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 2,
			dest, big.NewInt(5), big.NewInt(1), nil, nil)
		reveal.Salt = effectSalt
		if err := reveal.Signature(priv); err != nil {
			return err
		}

		commit := ngtypes.NewCommitment(ngtypes.ZERONET, 1, revealHash(reveal), big.NewInt(2))
		if err := commit.Signature(priv); err != nil {
			return err
		}

		// apply block@1 carrying the commitment: the fee (2) is charged
		block1 := blockWith(t, 1, minerAddr, []*ngtypes.Commitment{commit})
		if err := state.Upgrade(txn, block1); err != nil {
			t.Fatalf("apply commit block: %v", err)
		}
		if got := getBalance(txn, addr); got.Int64() != 98 {
			t.Fatalf("after commit fee, balance = %s, want 98", got)
		}
		if _, ok := findCommit(txn, addr, revealHash(reveal), 2); !ok {
			t.Fatal("commitment was not recorded for the reveal at height 2")
		}

		// the reveal is now admissible
		if err := CheckTx(txn, reveal); err != nil {
			t.Fatalf("committed reveal refused: %v", err)
		}

		// apply block@2 carrying the reveal: it executes (5 moves, 1 fee) and
		// consumes the commitment
		block2 := blockWith(t, 2, minerAddr, nil, reveal)
		if err := state.Upgrade(txn, block2); err != nil {
			t.Fatalf("apply reveal block: %v", err)
		}
		if got := getBalance(txn, dest); got.Int64() != 5 {
			t.Fatalf("recipient balance = %s, want 5", got)
		}
		if _, ok := findCommit(txn, addr, revealHash(reveal), 3); ok {
			t.Fatal("the reveal did not consume its commitment")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// blockWith builds a minimal block shell at the given height with a miner
// generate paying `miner`, plus optional commitments and effect txs. It is
// only fed to State.Upgrade (which consults height, txs and commits), never
// sealed, so the shell can borrow the genesis header.
func blockWith(t *testing.T, height uint64, miner ngtypes.Address, commits []*ngtypes.Commitment, txs ...*ngtypes.FullTx) *ngtypes.FullBlock {
	t.Helper()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	header := *genesis.BlockHeader
	header.Height = height
	header.Coinbase = miner[:]

	gen := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		miner, ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)

	all := append([]*ngtypes.FullTx{gen}, txs...)
	return &ngtypes.FullBlock{BlockHeader: &header, Txs: all, Commits: commits}
}
