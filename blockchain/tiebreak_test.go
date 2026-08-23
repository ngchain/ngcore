package blockchain_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngtypes"
)

// TestDeterministicTieBreak: on EQUAL work the smaller tip hash wins, so the
// outcome is independent of arrival order — every node converges on one tip.
func TestDeterministicTieBreak(t *testing.T) {
	minerA, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, minerA)
	a2 := mineBlock(t, b1, minerA)

	winner := mineWinningCompetitor(t, b1, a2) // hash < a2
	loser := mineLosingCompetitor(t, b1, a2)   // hash > a2

	// a lower-hash competitor displaces the current tip (even arriving second)
	c1 := newTestChain(t)
	mustApply(t, c1, b1)
	mustApply(t, c1, a2)
	mustApply(t, c1, winner)
	if !bytes.Equal(c1.GetLatestBlockHash(), winner.GetHash()) {
		t.Fatalf("lower-hash competitor must win the tie: tip=%x want=%x", c1.GetLatestBlockHash(), winner.GetHash())
	}

	// a higher-hash competitor never displaces the current tip
	c2 := newTestChain(t)
	mustApply(t, c2, b1)
	mustApply(t, c2, a2)
	mustApply(t, c2, loser)
	if !bytes.Equal(c2.GetLatestBlockHash(), a2.GetHash()) {
		t.Fatalf("higher-hash competitor must not displace the tip: tip=%x want=%x", c2.GetLatestBlockHash(), a2.GetHash())
	}
}

// TestTieBreakInvalidReorgRollsBackClean: a tie-break reorg toward an
// equal-work but INVALID competitor (lower hash) must fail and roll back to a
// fully consistent db.
func TestTieBreakInvalidReorgRollsBackClean(t *testing.T) {
	minerA, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, minerA)
	a2 := mineBlock(t, b1, minerA)

	// an over-reward (invalid) competitor with a hash lower than a2, so the
	// tie-break actually attempts the reorg toward it
	doubled := new(big.Int).Mul(ngtypes.GetBlockReward(2), big.NewInt(2))
	var bad *ngtypes.FullBlock
	for i := 0; i < 400; i++ {
		k, _ := ngtypes.GenerateKey()
		cand := mineBlockReward(t, b1, k, doubled)
		if bytes.Compare(cand.GetHash(), a2.GetHash()) < 0 {
			bad = cand
			break
		}
	}
	if bad == nil {
		t.Skip("could not mine a lower-hash invalid competitor")
	}

	chain := newTestChain(t)
	mustApply(t, chain, b1)
	mustApply(t, chain, a2)

	// the tie-break reorg attempt must fail on the invalid tx and roll back
	if err := chain.ApplyBlock(bad); err == nil {
		t.Fatal("reorg toward an over-reward competitor must fail")
	}
	if !bytes.Equal(chain.GetLatestBlockHash(), a2.GetHash()) {
		t.Fatalf("tip must stay a2 after the failed tie-break reorg: %x", chain.GetLatestBlockHash())
	}
	chain.CheckHealth(ngtypes.ZERONET) // db must be fully walkable
}

func mustApply(t *testing.T, chain *blockchain.Chain, b *ngtypes.FullBlock) {
	t.Helper()
	if err := chain.ApplyBlock(b); err != nil {
		t.Fatalf("apply block@%d: %v", b.GetHeight(), err)
	}
}
