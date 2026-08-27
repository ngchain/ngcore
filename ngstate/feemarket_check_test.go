package ngstate

import (
	"errors"
	"math/big"
	"testing"

	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// The per-tx burn-only base-fee minimum (ForkFeeMarket) is enforced by
// CheckBlockTxs from genesis on the dev networks (the fork is active at height
// 0): a reveal paying below block.BaseFee*bytes is rejected at every height
// (there is no pre-fork window to be lenient in any more), while a reveal paying
// at or above the minimum is accepted.
func TestCheckBlockTxsBaseFeeGate(t *testing.T) {
	db := newTestDB(t)

	minerPriv, _ := ngtypes.GenerateKey()
	minerAddr := ngtypes.NewAddress(minerPriv)
	userPriv, _ := ngtypes.GenerateKey()
	userAddr := ngtypes.NewAddress(userPriv)

	// build a block shell carrying MinBaseFee (every pre/post-fork block does)
	// at an arbitrary height, with a valid miner generate + the given effect txs
	blockAt := func(t *testing.T, height uint64, txs ...*ngtypes.FullTx) *ngtypes.FullBlock {
		genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
		header := *genesis.BlockHeader
		header.Height = height
		header.Coinbase = minerAddr[:]
		header.BaseFee = ngtypes.MinBaseFee.Bytes()

		gen := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
			minerAddr, ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
		if err := gen.Signature(minerPriv); err != nil {
			t.Fatal(err)
		}
		return &ngtypes.FullBlock{BlockHeader: &header, Txs: append([]*ngtypes.FullTx{gen}, txs...)}
	}

	// a signed transact reveal at a chosen height with a chosen fee, its
	// commitment seeded one block earlier (in window, strictly earlier)
	revealAt := func(t *testing.T, txn *bbolt.Tx, height uint64, fee *big.Int) *ngtypes.FullTx {
		tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, height,
			minerAddr, big.NewInt(1), fee, nil, nil)
		tx.Salt = effectSalt
		if err := tx.Signature(userPriv); err != nil {
			t.Fatal(err)
		}
		if err := putCommit(txn, height-1, revealHash(tx), userAddr); err != nil {
			t.Fatalf("seed commit: %v", err)
		}
		return tx
	}

	// minFeeFor is the exact post-fork minimum: MinBaseFee * len(rlp(tx))
	minFeeFor := func(tx *ngtypes.FullTx) *big.Int {
		raw, err := rlp.EncodeToBytes(tx)
		if err != nil {
			t.Fatal(err)
		}
		return new(big.Int).Mul(ngtypes.MinBaseFee, big.NewInt(int64(len(raw))))
	}

	err := db.Update(func(txn *bbolt.Tx) error {
		// fund the user generously so affordability never masks the fee gate
		if err := setBalance(txn, nil, userAddr, new(big.Int).Mul(ngtypes.NG, big.NewInt(100))); err != nil {
			return err
		}

		// The fork is active from genesis, so the lowest height a reveal can sit
		// at (its commit needs height-1) is already gated. Use height 1.
		const postHeight = ngtypes.FeeMarketForkHeight + 1

		// GENESIS-ACTIVE: a zero-fee reveal is rejected for paying below the base
		// fee, even at the lowest usable height (there is no lenient pre-fork window)
		postZero := revealAt(t, txn, postHeight, big.NewInt(0))
		err := CheckBlockTxs(txn, blockAt(t, postHeight, postZero))
		if !errors.Is(err, ngtypes.ErrTxFeeBelowBaseFee) {
			t.Fatalf("post-fork zero-fee reveal: got %v, want ErrTxFeeBelowBaseFee", err)
		}

		// POST-FORK: a reveal paying comfortably above MinBaseFee*bytes is
		// accepted (generous fee avoids the fee-length fixpoint)
		high := revealAt(t, txn, postHeight, new(big.Int).Mul(ngtypes.MinBaseFee, big.NewInt(4096)))
		if got := high.Fee.Cmp(minFeeFor(high)); got < 0 {
			t.Fatal("test setup: high fee is not above the minimum")
		}
		if err := CheckBlockTxs(txn, blockAt(t, postHeight, high)); err != nil {
			t.Fatalf("post-fork above-base-fee reveal must be accepted, got: %v", err)
		}

		// POST-FORK: just below the minimum (computed on the FINAL signed tx) is
		// rejected. Sign with a placeholder fee, then compute the true min from
		// the signed form and set fee = min-1 WITHOUT changing the encoded length
		// (min-1 has the same big-endian byte count as min for these sizes).
		below := revealAt(t, txn, postHeight, big.NewInt(0))
		below.Fee = new(big.Int).Sub(minFeeFor(below), big.NewInt(1))
		if err := below.Signature(userPriv); err != nil {
			return err
		}
		if err := putCommit(txn, postHeight-1, revealHash(below), userAddr); err != nil {
			return err
		}
		// recompute the min on the final signed form and confirm we are below it
		if below.Fee.Cmp(minFeeFor(below)) >= 0 {
			t.Fatalf("test setup: below.Fee %s is not under the min %s", below.Fee, minFeeFor(below))
		}
		if err := CheckBlockTxs(txn, blockAt(t, postHeight, below)); !errors.Is(err, ngtypes.ErrTxFeeBelowBaseFee) {
			t.Fatalf("post-fork just-below-minimum reveal: got %v, want ErrTxFeeBelowBaseFee", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
