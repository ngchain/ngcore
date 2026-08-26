package blockchain_test

import (
	"errors"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// TestApplyBlockWrongStateRootRejected: a block whose committed StateRoot does
// not match the post-state produced by applying it is rejected, and the reject
// happens inside the apply txn so the tip does not move.
func TestApplyBlockWrongStateRootRejected(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	b1 := mineBlock(t, genesis, miner)
	// corrupt the committed root and RE-SEAL so the pow still matches the
	// (now wrong-root) preimage — otherwise CheckError's nonce check would
	// fire first and mask the state-root rejection
	b1.BlockHeader.StateRoot = utils.Hash256([]byte("not the real root"))
	sealed := false
	for n := uint64(0); n < 1_000_000; n++ {
		if err := b1.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if b1.CheckError() == nil {
			sealed = true
			break
		}
	}
	if !sealed {
		t.Fatal("could not re-seal the corrupted block")
	}

	err := chain.ApplyBlock(b1)
	if !errors.Is(err, ngtypes.ErrBlockStateRootInvalid) {
		t.Fatalf("wrong state root: got %v, want ErrBlockStateRootInvalid", err)
	}
	if h := chain.GetLatestBlockHeight(); h != 0 {
		t.Fatalf("tip moved to %d despite a rejected block", h)
	}
}

// TestReorgKeepsStateRootsConsistent: a heavier competing branch that reorgs
// the chain must have every block's committed StateRoot reproduced by the
// replay, so the switch commits cleanly and the tip lands on the branch.
func TestReorgKeepsStateRootsConsistent(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	// canonical A: genesis <- a1 <- a2
	a1 := mineBlock(t, genesis, minerA)
	mustApply(t, chain, a1)
	a2 := mineBlock(t, a1, minerA)
	mustApply(t, chain, a2)

	// heavier branch B off a1: b2 <- b3 (3 blocks of work vs 2)
	b2 := mineBlock(t, a1, minerB)
	mustApply(t, chain, b2) // stored as a side block (equal work so far)
	b3 := mineBlock(t, b2, minerB)
	if err := chain.ApplyBlock(b3); err != nil {
		t.Fatalf("apply b3 (reorg trigger): %v", err)
	}

	if got := chain.GetLatestBlockHash(); string(got) != string(b3.GetHash()) {
		t.Fatalf("expected reorg to branch tip b3, got %x", got)
	}
}
