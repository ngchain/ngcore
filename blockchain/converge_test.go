package blockchain_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngtypes"
)

// TestConvergeRefusesLighterBranch guards the converge fork-choice fix: a
// branch fetched by the converge path is switched to ONLY if it carries
// strictly more cumulative work than the current tip (same rule as
// ApplyBlock). A shorter/lighter branch — e.g. one a remote advertised via a
// lucky checkpoint — must be refused, and a genuinely heavier one accepted.
func TestConvergeRefusesLighterBranch(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	b1 := mineBlock(t, genesis, minerA)
	mustApply(t, chain, b1)
	b2 := mineBlock(t, b1, minerA)
	mustApply(t, chain, b2)
	b3 := mineBlock(t, b2, minerA)
	mustApply(t, chain, b3) // local tip: 3 blocks past genesis

	// a shorter competing branch off b1 (one block) is lighter
	c2 := mineBlock(t, b1, minerB)
	if err := chain.SwitchToBranch([]*ngtypes.FullBlock{c2}); !errors.Is(err, blockchain.ErrBranchNotHeavier) {
		t.Fatalf("converge to a lighter branch: got %v, want ErrBranchNotHeavier", err)
	}
	if !bytes.Equal(chain.GetLatestBlockHash(), b3.GetHash()) {
		t.Fatal("tip must stay on the heavier local chain")
	}

	// a longer competing branch off b1 (three blocks) is heavier and wins
	d2 := mineBlock(t, b1, minerB)
	d3 := mineBlock(t, d2, minerB)
	d4 := mineBlock(t, d3, minerB)
	if err := chain.SwitchToBranch([]*ngtypes.FullBlock{d2, d3, d4}); err != nil {
		t.Fatalf("converge to a heavier branch was rejected: %v", err)
	}
	if !bytes.Equal(chain.GetLatestBlockHash(), d4.GetHash()) {
		t.Fatal("tip must move to the heavier branch")
	}
}
