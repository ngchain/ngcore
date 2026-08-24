package blockchain_test

import (
	"bytes"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

// These tests simulate the classic selfish-mining moves and assert the fork
// choice neutralizes them. Selfish mining is profitable mainly because the
// attacker can (a) win same-height ties via network position (first-seen /
// high γ), (b) waste honest work, and (c) convert a private lead cheaply.
// The defenses here are: a DETERMINISTIC hash tie-break (γ is a coin flip,
// not gameable), CUMULATIVE-WORK fork choice (no winning with less work),
// and uncles being reward-only (no stealing honest work as weight).

// TestSelfishMiningNoFirstSeenAdvantage: the attacker privately mines a
// competitor at the same height and releases it LATE, after the honest block
// is already every node's tip — the classic withhold-and-release. With a
// deterministic hash tie-break the release ORDER is irrelevant: every node
// converges on the smaller-hash block, so honest hashpower never splits and
// the attacker's win probability is exactly 0.5 (hash luck), never boosted by
// being better-connected or releasing at the right moment.
func TestSelfishMiningNoFirstSeenAdvantage(t *testing.T) {
	honest, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	// two nodes that will see the two blocks in OPPOSITE orders
	nodeSeesHonestFirst := newTestChain(t)
	nodeSeesAttackerFirst := newTestChain(t)

	h1 := mineBlock(t, genesis, honest)
	mustApply(t, nodeSeesHonestFirst, h1)
	mustApply(t, nodeSeesAttackerFirst, h1)

	// honest block at height 2, and the attacker's withheld competitor with a
	// SMALLER hash (the attacker got lucky this round)
	honestB2 := mineBlock(t, h1, honest)
	attackerB2 := mineWinningCompetitor(t, h1, honestB2) // hash < honestB2

	// node A: honest arrives first (becomes tip), attacker released LATE
	mustApply(t, nodeSeesHonestFirst, honestB2)
	mustApply(t, nodeSeesHonestFirst, attackerB2)

	// node B: attacker arrives first, honest arrives late
	mustApply(t, nodeSeesAttackerFirst, attackerB2)
	mustApply(t, nodeSeesAttackerFirst, honestB2)

	// both nodes converge on the SAME tip regardless of arrival order — honest
	// hashpower does not split, and it is the smaller-hash block that wins, not
	// the first-seen one
	tipA := nodeSeesHonestFirst.GetLatestBlockHash()
	tipB := nodeSeesAttackerFirst.GetLatestBlockHash()
	if !bytes.Equal(tipA, tipB) {
		t.Fatal("nodes diverged on arrival order — first-seen advantage exists (selfish mining works)")
	}
	if !bytes.Equal(tipA, attackerB2.GetHash()) {
		t.Fatal("the smaller-hash block did not win the tie deterministically")
	}

	// and the mirror: a LARGER-hash withheld block loses even when released
	// FIRST — timing cannot rescue it
	loserNode := newTestChain(t)
	mustApply(t, loserNode, h1)
	attackerHighHash := mineLosingCompetitor(t, h1, honestB2) // hash > honestB2
	mustApply(t, loserNode, attackerHighHash)                 // attacker first
	mustApply(t, loserNode, honestB2)                         // honest late
	if !bytes.Equal(loserNode.GetLatestBlockHash(), honestB2.GetHash()) {
		t.Fatal("a larger-hash attacker block won by releasing first — timing advantage exists")
	}
}

// TestSelfishMiningCannotWinWithLessWork: an attacker who secretly mined a
// SHORTER private fork cannot override the honest chain by releasing it. Fork
// choice is strictly heaviest-work, so withholding a minority fork is wasted.
func TestSelfishMiningCannotWinWithLessWork(t *testing.T) {
	chain := newTestChain(t)
	honest, _ := ngtypes.GenerateKey()
	attacker, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	// honest public chain: 3 blocks past the fork point h1
	h1 := mineBlock(t, genesis, honest)
	mustApply(t, chain, h1)
	h2 := mineBlock(t, h1, honest)
	mustApply(t, chain, h2)
	h3 := mineBlock(t, h2, honest)
	mustApply(t, chain, h3)
	h4 := mineBlock(t, h3, honest)
	mustApply(t, chain, h4)

	// attacker secretly built only 2 blocks off h1, then releases them
	a2 := mineBlock(t, h1, attacker)
	mustApply(t, chain, a2)
	a3 := mineBlock(t, a2, attacker)
	mustApply(t, chain, a3)

	if !bytes.Equal(chain.GetLatestBlockHash(), h4.GetHash()) {
		t.Fatal("a shorter private fork overrode the heavier honest chain")
	}
}

// TestSelfishMiningLeadReleaseIsOnlyATie: the core selfish move is to keep a
// one-block lead private and release it the instant the honest chain catches
// up, hoping to orphan the honest block. Here the attacker's released chain
// only TIES the honest one at that height, so the outcome is the deterministic
// coin-flip tie-break — not a guaranteed steal. The attacker cannot convert a
// mere lead into a certain win; it needs strictly more work (i.e. > 50%).
func TestSelfishMiningLeadReleaseIsOnlyATie(t *testing.T) {
	chain := newTestChain(t)
	honest, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	h1 := mineBlock(t, genesis, honest)
	mustApply(t, chain, h1)

	// honest publishes height 2; the attacker had privately mined a competitor
	// at height 2 that LOSES the tie-break (higher hash)
	honestB2 := mineBlock(t, h1, honest)
	mustApply(t, chain, honestB2)
	attackerB2 := mineLosingCompetitor(t, h1, honestB2)
	mustApply(t, chain, attackerB2)

	// the attacker's equal-height private block does not steal the tip: with a
	// losing hash, withholding bought nothing
	if !bytes.Equal(chain.GetLatestBlockHash(), honestB2.GetHash()) {
		t.Fatal("a losing-hash withheld block still stole the tip")
	}
}

// TestSelfishMiningMajorityStillWins is the sanity bound: the defense sits at
// the 50% line, it does not over-defend. A genuinely heavier (more-work)
// chain — what a > 50% attacker would build — MUST win, otherwise the chain
// could not make progress or reorg to honest majorities.
func TestSelfishMiningMajorityStillWins(t *testing.T) {
	chain := newTestChain(t)
	honest, _ := ngtypes.GenerateKey()
	attacker, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	h1 := mineBlock(t, genesis, honest)
	mustApply(t, chain, h1)
	h2 := mineBlock(t, h1, honest)
	mustApply(t, chain, h2)

	// attacker's heavier fork off h1 (3 blocks vs the honest 1 block past h1)
	a2 := mineBlock(t, h1, attacker)
	mustApply(t, chain, a2)
	a3 := mineBlock(t, a2, attacker)
	mustApply(t, chain, a3)
	a4 := mineBlock(t, a3, attacker)
	mustApply(t, chain, a4)

	if !bytes.Equal(chain.GetLatestBlockHash(), a4.GetHash()) {
		t.Fatal("a strictly heavier chain failed to win — fork choice is broken")
	}
}
