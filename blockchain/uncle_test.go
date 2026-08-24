package blockchain_test

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// mineBlockWithUncles seals a ZERONET block on parent that references the
// given uncle headers. Uncles are attached BEFORE sealing, so the pow
// covers the uncle commitment (as the miner path does).
func mineBlockWithUncles(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey, uncles []*ngtypes.BlockHeader) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16
	diff := ngtypes.GetNextDiff(height, blockTime, parent)
	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(), diff)
	block.SetCoinbase(ngtypes.NewAddress(miner))

	block.SetUncles(uncles) // must precede sealing: UnclesHash is in the pow preimage

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner), ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	// each referenced uncle needs its (unsigned) reward generate
	txs := []*ngtypes.FullTx{genTx}
	for _, u := range uncles {
		var to ngtypes.Address
		copy(to[:], u.Coinbase)
		txs = append(txs, ngtypes.NewUnsignedTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
			to, ngtypes.UncleReward(u.Height, height), big.NewInt(0), nil))
	}
	if err := block.ToUnsealing(txs); err != nil {
		t.Fatal(err)
	}

	for n := uint64(0); n < 1_000_000; n++ {
		if err := block.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if block.CheckError() == nil {
			return block
		}
	}
	t.Fatal("failed to seal an uncle-carrying block")
	return nil
}

// TestUncleWorkDoesNotAffectForkChoice guards the fix for a sub-50% reorg:
// uncle difficulty must NOT count toward fork-choice weight. If it did, a
// block could out-weigh an equal-real-work sibling merely by referencing an
// orphan (and an attacker could reference the honest chain's own blocks).
// So a lower-hash plain block must still reorg out a higher-hash sibling
// that carries an uncle — the tie-break, not the uncle, decides equal work.
func TestUncleWorkDoesNotAffectForkChoice(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	b1 := mineBlock(t, genesis, minerA)
	mustApply(t, chain, b1)
	b2 := mineBlock(t, b1, minerA)
	mustApply(t, chain, b2)

	orphan := mineLosingCompetitor(t, b1, b2)
	mustApply(t, chain, orphan)

	// an uncle-carrying block takes the tip first
	withUncle := mineBlockWithUncles(t, b2, minerB, []*ngtypes.BlockHeader{orphan.BlockHeader})
	mustApply(t, chain, withUncle)
	if !bytes.Equal(chain.GetLatestBlockHash(), withUncle.GetHash()) {
		t.Fatal("uncle-carrying block should be the tip")
	}

	// a plain sibling with a SMALLER hash and no uncle: equal real work, so
	// the tie-break adopts it. If uncle work counted, withUncle would be
	// heavier and this reorg would (wrongly) not happen.
	plain := mineWinningCompetitor(t, b2, withUncle)
	if err := chain.ApplyBlock(plain); err != nil {
		t.Fatalf("plain competitor rejected: %v", err)
	}
	if !bytes.Equal(chain.GetLatestBlockHash(), plain.GetHash()) {
		t.Fatal("uncle work must not out-weigh an equal-real-work, lower-hash sibling")
	}
}

// TestUncleRewardPaysOrphanMiner: referencing an orphan credits its miner
// (the header Coinbase) exactly the depth-decayed UncleReward.
func TestUncleRewardPaysOrphanMiner(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	b1 := mineBlock(t, genesis, minerA)
	mustApply(t, chain, b1)
	b2 := mineBlock(t, b1, minerA)
	mustApply(t, chain, b2)
	orphan := mineLosingCompetitor(t, b1, b2)
	mustApply(t, chain, orphan)

	var orphanAddr ngtypes.Address
	copy(orphanAddr[:], orphan.BlockHeader.Coinbase)
	before, err := chain.State.GetTotalBalanceByAddress(orphanAddr)
	if err != nil {
		t.Fatal(err)
	}

	nephew := mineBlockWithUncles(t, b2, minerB, []*ngtypes.BlockHeader{orphan.BlockHeader})
	mustApply(t, chain, nephew)

	after, err := chain.State.GetTotalBalanceByAddress(orphanAddr)
	if err != nil {
		t.Fatal(err)
	}
	want := ngtypes.UncleReward(orphan.GetHeight(), nephew.GetHeight())
	if want.Sign() == 0 {
		t.Fatal("test setup: uncle reward should be non-zero at depth 1")
	}
	if got := new(big.Int).Sub(after, before); got.Cmp(want) != 0 {
		t.Fatalf("orphan miner credited %s, want %s", got, want)
	}
}

// TestUncleRewardWrongAmountRejected: a nephew that over-pays an uncle is
// refused by the generate-set validation.
func TestUncleRewardWrongAmountRejected(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	b1 := mineBlock(t, genesis, minerA)
	mustApply(t, chain, b1)
	b2 := mineBlock(t, b1, minerA)
	mustApply(t, chain, b2)
	orphan := mineLosingCompetitor(t, b1, b2)
	mustApply(t, chain, orphan)

	height := b2.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16
	bad := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, b2.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, b2))
	bad.SetCoinbase(ngtypes.NewAddress(minerB))
	bad.SetUncles([]*ngtypes.BlockHeader{orphan.BlockHeader})

	gen := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(minerB), ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
	if err := gen.Signature(minerB); err != nil {
		t.Fatal(err)
	}
	var to ngtypes.Address
	copy(to[:], orphan.BlockHeader.Coinbase)
	inflated := new(big.Int).Add(ngtypes.UncleReward(orphan.GetHeight(), height), ngtypes.NG)
	badReward := ngtypes.NewUnsignedTx(ngtypes.ZERONET, ngtypes.GenerateTx, height, to, inflated, big.NewInt(0), nil)
	if err := bad.ToUnsealing([]*ngtypes.FullTx{gen, badReward}); err != nil {
		t.Fatal(err)
	}
	for n := uint64(0); n < 1_000_000; n++ {
		if err := bad.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if bad.CheckError() == nil {
			break
		}
	}

	if err := chain.ApplyBlock(bad); !errors.Is(err, ngtypes.ErrRewardInvalid) {
		t.Fatalf("inflated uncle reward: got %v, want ErrRewardInvalid", err)
	}
}

// TestUncleOutOfDepthRejected: an uncle whose fork point is deeper than
// UncleMaxDepth generations back is rejected.
func TestUncleOutOfDepthRejected(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	b1 := mineBlock(t, genesis, miner)
	mustApply(t, chain, b1)
	b2 := mineBlock(t, b1, miner)
	mustApply(t, chain, b2)

	orphan := mineLosingCompetitor(t, b1, b2) // height 2 side block
	mustApply(t, chain, orphan)

	// extend the canonical chain until referencing the height-2 orphan would
	// exceed UncleMaxDepth
	parent := b2
	for parent.GetHeight() < orphan.GetHeight()+uint64(ngtypes.UncleMaxDepth)+1 {
		parent = mineBlock(t, parent, miner)
		mustApply(t, chain, parent)
	}

	bad := mineBlockWithUncles(t, parent, miner, []*ngtypes.BlockHeader{orphan.BlockHeader})
	if err := chain.ApplyBlock(bad); !errors.Is(err, blockchain.ErrUncleInvalid) {
		t.Fatalf("out-of-depth uncle: got %v, want ErrUncleInvalid", err)
	}
}

// TestUncleOnChainAncestorRejected: a block cannot reference one of its own
// canonical ancestors as an uncle (that work is already counted).
func TestUncleOnChainAncestorRejected(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	b1 := mineBlock(t, genesis, miner)
	mustApply(t, chain, b1)
	b2 := mineBlock(t, b1, miner)
	mustApply(t, chain, b2)

	// b3 tries to claim its grandparent b1 (a canonical ancestor) as an uncle
	bad := mineBlockWithUncles(t, b2, miner, []*ngtypes.BlockHeader{b1.BlockHeader})
	if err := chain.ApplyBlock(bad); !errors.Is(err, blockchain.ErrUncleInvalid) {
		t.Fatalf("on-chain-ancestor uncle: got %v, want ErrUncleInvalid", err)
	}
}
