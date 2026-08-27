package e2e

import (
	"math/big"
	"testing"

	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// every network's genesis header carries exactly MinBaseFee.
func TestGenesisCarriesMinBaseFee(t *testing.T) {
	for _, net := range ngtypes.AvailableNetworks {
		g := ngtypes.GetGenesisBlock(net)
		got := new(big.Int).SetBytes(g.BlockHeader.BaseFee)
		if got.Cmp(ngtypes.MinBaseFee) != 0 {
			t.Fatalf("%s genesis BaseFee = %s, want MinBaseFee %s", net, got, ngtypes.MinBaseFee)
		}
	}
}

// mineChainTo mines empty blocks on the node's tip up to (and including) the
// target height, submitting each through the full import path.
func mineChainTo(t *testing.T, node *testNode, miner *ngtypes.PrivateKey, target uint64) {
	t.Helper()
	for node.chain.GetLatestBlockHeight() < target {
		mineAndSubmit(t, node, miner)
	}
}

// crossing the fork boundary: below FeeMarketForkHeight the per-tx base-fee
// minimum is NOT enforced (a zero-fee reveal lands) and every header carries
// MinBaseFee; at/above the fork height the header base fee begins updating and
// a reveal paying below BaseFee*bytes is rejected by the state gate.
func TestFeeMarketForkBoundary(t *testing.T) {
	node := newNode(t)
	miner, _ := ngtypes.GenerateKey()

	// fund the sender and reach one block below the fork boundary
	sender := miner // the miner mines its own funds
	mineChainTo(t, node, miner, ngtypes.FeeMarketForkHeight-1)

	// every header so far carries MinBaseFee (pre-fork NextBaseFee is constant)
	for h := uint64(0); h <= node.chain.GetLatestBlockHeight(); h++ {
		b := blockAtHeight(t, node, h)
		if got := new(big.Int).SetBytes(b.BlockHeader.BaseFee); got.Cmp(ngtypes.MinBaseFee) != 0 {
			t.Fatalf("pre-fork block@%d BaseFee = %s, want MinBaseFee", h, got)
		}
	}

	// PRE-FORK: a zero-fee reveal is admissible into a block. Build the commit at
	// the current tip height, reveal one higher — both still below the fork.
	commitHeight := node.chain.GetLatestBlockHeight() + 1
	if !ngtypes.IsForkActive(ngtypes.ZERONET, ngtypes.ForkFeeMarket, commitHeight+1) {
		// good: the reveal height is still pre-fork
		var payee ngtypes.Address
		payee[0] = 0x77
		reveal := revealTx(t, ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, commitHeight+1,
			payee, big.NewInt(1), big.NewInt(0), nil, nil), sender)
		commit := commitFor(t, reveal, sender, commitHeight, nil)

		tip := node.chain.GetLatestBlock().(*ngtypes.FullBlock)
		cb := mineOnAll(t, tip, miner, []*ngtypes.Commitment{commit})
		if err := node.pow.MinedNewBlock(cb); err != nil {
			t.Fatalf("mine pre-fork commit block: %v", err)
		}
		rb := mineOnTxs(t, cb, miner, reveal)
		if err := node.pow.MinedNewBlock(rb); err != nil {
			t.Fatalf("pre-fork zero-fee reveal must be accepted, got: %v", err)
		}
		if got, _ := node.chain.State.GetTotalBalanceByAddress(payee); got.Sign() == 0 {
			t.Fatal("pre-fork zero-fee transfer did not credit the payee")
		}
	}

	// advance to the fork height
	mineChainTo(t, node, miner, ngtypes.FeeMarketForkHeight)
	if !ngtypes.IsForkActive(ngtypes.ZERONET, ngtypes.ForkFeeMarket, node.chain.GetLatestBlockHeight()) {
		t.Fatalf("expected the fork active at height %d", node.chain.GetLatestBlockHeight())
	}

	// POST-FORK: a reveal paying below BaseFee*bytes must be rejected by the
	// block's tx gate. Build a well-formed reveal with fee 0 and mine it into a
	// post-fork block; ApplyBlock must reject the block.
	postCommitHeight := node.chain.GetLatestBlockHeight() + 1
	postRevealHeight := postCommitHeight + 1 // post-fork
	var dst ngtypes.Address
	dst[0] = 0x88
	lowFee := revealTx(t, ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, postRevealHeight,
		dst, big.NewInt(1), big.NewInt(0), nil, nil), sender)
	// the commitment must clear the pool AND carry enough fee to be affordable;
	// mine it directly (bypassing the pool) into a block so the reveal is on-chain
	commit := commitFor(t, lowFee, sender, postCommitHeight, ngtypes.MinBaseFee)

	tip := node.chain.GetLatestBlock().(*ngtypes.FullBlock)
	cb := mineOnAll(t, tip, miner, []*ngtypes.Commitment{commit})
	if err := node.pow.MinedNewBlock(cb); err != nil {
		t.Fatalf("mine post-fork commit block: %v", err)
	}

	// assemble the reveal block by hand (mineOnTxs seals it), then apply — the
	// state gate must reject the zero-fee reveal for paying below the base fee
	rb := mineOnTxs(t, cb, miner, lowFee)
	if err := node.pow.MinedNewBlock(rb); err == nil {
		t.Fatal("post-fork zero-fee reveal must be rejected (below base fee), but the block was accepted")
	}
}

// a post-fork block whose header BaseFee != the consensus-computed NextBaseFee
// is rejected by the chain layer (exact-match, like difficulty).
func TestFeeMarketHeaderBaseFeeMismatchRejected(t *testing.T) {
	node := newNode(t)
	miner, _ := ngtypes.GenerateKey()

	mineChainTo(t, node, miner, ngtypes.FeeMarketForkHeight)

	tip := node.chain.GetLatestBlock().(*ngtypes.FullBlock)

	// build a valid next block, then tamper the header BaseFee and re-seal so the
	// pow preimage stays consistent with the tampered value; the chain must still
	// reject it because BaseFee != NextBaseFee(parent).
	bad := buildTamperedBaseFeeBlock(t, tip, miner)
	if err := node.chain.CheckBlock(bad); err == nil {
		t.Fatal("a block with a wrong header BaseFee must be rejected, but CheckBlock accepted it")
	}
}

// buildTamperedBaseFeeBlock mines a valid block on parent, then rewrites its
// BaseFee to a wrong value and re-seals (so pow is valid for the tampered
// header). Only the base-fee-correctness check should reject it.
func buildTamperedBaseFeeBlock(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16

	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, parent))
	block.SetCoinbase(ngtypes.NewAddress(miner))

	// the correct base fee is NextBaseFee(parent); pick a DIFFERENT valid-length
	// value (correct + 1) so only the equality check fails
	correct := ngtypes.NextBaseFee(ngtypes.ZERONET, parent.GetHeight(),
		new(big.Int).SetBytes(parent.BlockHeader.BaseFee), ngtypes.BlockUsedBytes(parent))
	block.BlockHeader.BaseFee = new(big.Int).Add(correct, big.NewInt(1)).Bytes()

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner), ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	if err := block.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
		t.Fatal(err)
	}
	sealStateRoot(t, block)

	for n := uint64(0); n < 1_000_000; n++ {
		if err := block.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if block.CheckError() == nil { // context-free checks pass; only the chain check fails
			registerBuilt(block)
			return block
		}
	}
	t.Fatal("failed to seal tampered block")
	return nil
}

// blockAtHeight loads a canonical block by height from the node's chain.
func blockAtHeight(t *testing.T, node *testNode, height uint64) *ngtypes.FullBlock {
	t.Helper()
	b, err := node.chain.GetBlockByHeight(height)
	if err != nil {
		t.Fatalf("load block@%d: %v", height, err)
	}
	return b.(*ngtypes.FullBlock)
}

// the header base fee actually moves once the fork is active: a full post-fork
// block raises the child's base fee above MinBaseFee.
func TestFeeMarketBaseFeeMovesWhenFull(t *testing.T) {
	// unit-level: a full parent post-fork yields a child base fee above the floor
	parentBaseFee := new(big.Int).Set(ngtypes.MinBaseFee)
	child := ngtypes.NextBaseFee(ngtypes.ZERONET, ngtypes.FeeMarketForkHeight,
		parentBaseFee, uint64(ngtypes.MaxBlockBytes))
	if child.Cmp(ngtypes.MinBaseFee) <= 0 {
		t.Fatalf("full post-fork block: child base fee %s did not rise above MinBaseFee %s",
			child, ngtypes.MinBaseFee)
	}
	// and rlp round-trips the new BaseFee field on a header carrying it
	h := &ngtypes.BlockHeader{
		Network: ngtypes.ZERONET, Height: ngtypes.FeeMarketForkHeight,
		PrevBlockHash: make([]byte, 32), TxTrieHash: make([]byte, 32),
		WitnessRoot: make([]byte, 32), Difficulty: big.NewInt(1).Bytes(),
		Coinbase: make([]byte, 32), UnclesHash: make([]byte, 32),
		StateRoot: make([]byte, 32), BaseFee: child.Bytes(), Nonce: make([]byte, 8),
	}
	raw, err := rlp.EncodeToBytes(h)
	if err != nil {
		t.Fatal(err)
	}
	var back ngtypes.BlockHeader
	if err := rlp.DecodeBytes(raw, &back); err != nil {
		t.Fatal(err)
	}
	if new(big.Int).SetBytes(back.BaseFee).Cmp(child) != 0 {
		t.Fatalf("BaseFee did not survive rlp round-trip: %x", back.BaseFee)
	}
}
