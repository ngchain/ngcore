package blockchain_test

import (
	"bytes"
	"errors"
	"math/big"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

func TestCheckBlock(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	// the genesis block always passes
	if err := chain.CheckBlock(genesis); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	b1 := mineBlock(t, genesis, miner)
	if err := chain.CheckBlock(b1); err != nil {
		t.Fatalf("valid b1: %v", err)
	}

	// an orphan whose prev block is unknown
	b2 := mineBlock(t, b1, miner)
	if err := chain.CheckBlock(b2); err == nil {
		t.Fatal("orphan must fail the check")
	}

	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}
	if err := chain.CheckBlock(b2); err != nil {
		t.Fatalf("b2 after b1: %v", err)
	}

	// a block declaring the wrong difficulty
	height := b1.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16
	wrongDiff := new(big.Int).Add(ngtypes.GetNextDiff(height, blockTime, b1), big.NewInt(1))
	bad := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, b1.GetHash(), wrongDiff)
	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner), ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	if err := bad.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
		t.Fatal(err)
	}
	for n := uint64(0); ; n++ {
		if err := bad.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if bad.CheckError() == nil {
			break
		}
		if n > 1_000_000 {
			t.Fatal("failed to seal the wrong-diff block")
		}
	}
	if err := chain.CheckBlock(bad); !errors.Is(err, ngtypes.ErrBlockDiffInvalid) {
		t.Fatalf("wrong declared diff: got %v, want ErrBlockDiffInvalid", err)
	}
}

func TestCheckHealth(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	parent := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	for i := 0; i < 3; i++ {
		parent = mineBlock(t, parent, miner)
		if err := chain.ApplyBlock(parent); err != nil {
			t.Fatal(err)
		}
	}

	// a healthy chain passes without panicking
	chain.CheckHealth(ngtypes.ZERONET)
}

func TestLatestAndOriginGetters(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	// on a fresh chain everything is the genesis block
	if !bytes.Equal(chain.GetLatestBlock().GetHash(), genesis.GetHash()) {
		t.Fatal("latest block should be genesis")
	}
	if !bytes.Equal(chain.GetOriginBlock().GetHash(), genesis.GetHash()) {
		t.Fatal("origin block should be genesis")
	}
	if !bytes.Equal(chain.GetLatestCheckpointHash(), genesis.GetHash()) {
		t.Fatal("latest checkpoint should be genesis")
	}

	b1 := mineBlock(t, genesis, miner)
	b2 := mineBlock(t, b1, miner)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(b2); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(chain.GetLatestBlock().GetHash(), b2.GetHash()) {
		t.Fatal("latest block should be b2")
	}
	if !bytes.Equal(chain.GetOriginBlock().GetHash(), genesis.GetHash()) {
		t.Fatal("origin must stay genesis")
	}

	// tip@2 is neither genesis nor a checkpoint: walk back to height 0
	cp := chain.GetLatestCheckpoint()
	if !bytes.Equal(cp.GetHash(), genesis.GetHash()) {
		t.Fatal("checkpoint below one round should be genesis")
	}
}

func TestGetLatestCheckpointAtRound(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	parent := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	var head *ngtypes.FullBlock
	for parent.GetHeight() < uint64(ngtypes.BlockCheckRound)+1 {
		parent = mineBlock(t, parent, miner)
		if err := chain.ApplyBlock(parent); err != nil {
			t.Fatal(err)
		}
		if parent.IsHead() {
			head = parent
		}
	}

	// tip@11: the checkpoint is the block at height 10
	cp := chain.GetLatestCheckpoint()
	if !bytes.Equal(cp.GetHash(), head.GetHash()) {
		t.Fatalf("checkpoint = %x@%d, want the head block@%d",
			cp.GetHash(), cp.GetHeight(), head.GetHeight())
	}

	// a tip sitting exactly on the round is its own checkpoint
	chain2 := newTestChain(t)
	parent = ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	for parent.GetHeight() < uint64(ngtypes.BlockCheckRound) {
		parent = mineBlock(t, parent, miner)
		if err := chain2.ApplyBlock(parent); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(chain2.GetLatestCheckpointHash(), parent.GetHash()) {
		t.Fatal("a head tip must be its own checkpoint")
	}
}

func TestGetBlockByHeightAndHashErrors(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, miner)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}

	// height 0 and the genesis hash are answered without the db
	if b, err := chain.GetBlockByHeight(0); err != nil || !b.(*ngtypes.FullBlock).IsGenesis() {
		t.Fatalf("block@0: %v, %v", b, err)
	}
	if _, err := chain.GetBlockByHash(genesis.GetHash()); err != nil {
		t.Fatalf("genesis by hash: %v", err)
	}

	if _, err := chain.GetBlockByHeight(42); err == nil {
		t.Fatal("missing height must error")
	}
	if _, err := chain.GetBlockByHash([]byte{0x01, 0x02}); !errors.Is(err, ngtypes.ErrHashSize) {
		t.Fatalf("short hash: got %v, want ErrHashSize", err)
	}
	if _, err := chain.GetBlockByHash(bytes.Repeat([]byte{0xaa}, 32)); err == nil {
		t.Fatal("unknown hash must error")
	}
}

func TestGettersOnEmptyDB(t *testing.T) {
	// a chain over a db with buckets but no genesis/tags: every getter
	// falls back gracefully instead of panicking
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "empty.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	storage.InitDB(db)

	chain := blockchain.Init(db, ngtypes.ZERONET, nil, nil)

	if hash := chain.GetLatestBlockHash(); hash != nil {
		t.Fatalf("latest hash = %x, want nil", hash)
	}
	if h := chain.GetLatestBlockHeight(); h != 0 {
		t.Fatalf("latest height = %d, want 0", h)
	}
	if b := chain.GetOriginBlock(); !b.IsGenesis() {
		t.Fatal("origin fallback must be genesis")
	}
	if b := chain.GetLatestBlock(); !b.(*ngtypes.FullBlock).IsGenesis() {
		t.Fatal("latest fallback must be genesis")
	}
}

func TestGetLatestBlockFallbackOnBrokenDB(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	b1 := mineBlock(t, ngtypes.GetGenesisBlock(ngtypes.ZERONET), miner)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}

	// break the db: the latest tags point at height 1 but the block
	// data behind the height index is gone
	if err := chain.DB.Update(func(txn *bbolt.Tx) error {
		return txn.Bucket(storage.BlockBucketName).Delete(b1.GetHash())
	}); err != nil {
		t.Fatal(err)
	}

	if b := chain.GetLatestBlock(); !b.(*ngtypes.FullBlock).IsGenesis() {
		t.Fatal("broken latest block must fall back to genesis")
	}
}

func TestGetTxByHash(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	b1 := mineBlock(t, ngtypes.GetGenesisBlock(ngtypes.ZERONET), miner)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}

	want := b1.Txs[0]
	got, err := chain.GetTxByHash(want.GetHash())
	if err != nil {
		t.Fatalf("get tx: %v", err)
	}
	if !bytes.Equal(got.GetHash(), want.GetHash()) {
		t.Fatal("tx hash mismatch")
	}

	if _, err := chain.GetTxByHash(bytes.Repeat([]byte{0xbb}, 32)); err == nil {
		t.Fatal("unknown tx must error")
	}
	if _, _, err := chain.GetTxLocation(bytes.Repeat([]byte{0xbb}, 32)); err == nil {
		t.Fatal("unknown tx location must error")
	}
}

func TestForceApplyBlocks(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, miner)
	b2 := mineBlock(t, b1, miner)

	if err := chain.ForceApplyBlocks([]*ngtypes.FullBlock{b1, b2}); err != nil {
		t.Fatalf("force apply: %v", err)
	}
	if h := chain.GetLatestBlockHeight(); h != 2 {
		t.Fatalf("height = %d, want 2", h)
	}
	if !bytes.Equal(chain.GetLatestBlockHash(), b2.GetHash()) {
		t.Fatal("tip should be b2")
	}

	// a segment whose first block has no stored prev is rejected
	chain2 := newTestChain(t)
	if err := chain2.ForceApplyBlocks([]*ngtypes.FullBlock{b2}); err == nil {
		t.Fatal("dangling segment must be rejected")
	}
}

func TestSwitchToBranchSuccess(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, minerA)
	a2 := mineBlock(t, b1, minerA)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(a2); err != nil {
		t.Fatal(err)
	}

	// a branch fetched from a remote replaces the canonical chain
	c2 := mineBlock(t, b1, minerB)
	c3 := mineBlock(t, c2, minerB)
	if err := chain.SwitchToBranch([]*ngtypes.FullBlock{c2, c3}); err != nil {
		t.Fatalf("switch: %v", err)
	}

	if !bytes.Equal(chain.GetLatestBlockHash(), c3.GetHash()) {
		t.Fatal("tip should be c3")
	}
	if got := balanceOf(t, chain, minerB); got.Sign() <= 0 {
		t.Fatal("the state must be replayed onto the branch")
	}

	// rejections: empty and internally disconnected branches
	if err := chain.SwitchToBranch(nil); !errors.Is(err, ngblocks.ErrBranchEmpty) {
		t.Fatalf("empty branch: got %v, want ErrBranchEmpty", err)
	}
	if err := chain.SwitchToBranch([]*ngtypes.FullBlock{c3, c2}); err == nil {
		t.Fatal("disconnected branch must be rejected")
	}
}

func TestSwitchToBranchOntoCheckpoint(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()

	// canonical chain to one block below the checkpoint round
	parent := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	for parent.GetHeight() < uint64(ngtypes.BlockCheckRound)-1 {
		parent = mineBlock(t, parent, minerA)
		if err := chain.ApplyBlock(parent); err != nil {
			t.Fatal(err)
		}
	}

	// a branch landing exactly on the checkpoint height: the switch
	// must also generate the snapshot and prune finalized side blocks
	c := mineBlock(t, parent, minerB)
	branch := []*ngtypes.FullBlock{c}
	if err := chain.SwitchToBranch(branch); err != nil {
		t.Fatalf("switch onto checkpoint: %v", err)
	}

	if h := chain.GetLatestBlockHeight(); h != uint64(ngtypes.BlockCheckRound) {
		t.Fatalf("height = %d, want %d", h, ngtypes.BlockCheckRound)
	}
	if sheet := chain.State.GetSnapshotByHeight(uint64(ngtypes.BlockCheckRound)); sheet == nil {
		t.Fatal("the checkpoint switch must persist a snapshot")
	}
}

func TestReorgFromGenesisFork(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, minerA)
	b2 := mineBlock(t, b1, minerA)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(b2); err != nil {
		t.Fatal(err)
	}

	// a branch forking at the ORIGIN itself (genesis)
	c1 := mineBlock(t, genesis, minerB)
	c2 := mineBlock(t, c1, minerB)
	c3 := mineBlock(t, c2, minerB)

	if err := chain.ApplyBlock(c1); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(c2); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(c3); err != nil {
		t.Fatalf("reorg from the genesis fork: %v", err)
	}

	if !bytes.Equal(chain.GetLatestBlockHash(), c3.GetHash()) {
		t.Fatal("tip should be c3")
	}
	canon1, err := chain.GetBlockByHeight(1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canon1.GetHash(), c1.GetHash()) {
		t.Fatal("height 1 should map to c1 after the reorg")
	}
}

func TestApplySnapshot(t *testing.T) {
	// chain A mines one full round and serves the checkpoint sheet
	chainA := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	parent := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	blocks := make([]*ngtypes.FullBlock, 0)
	for parent.GetHeight() < uint64(ngtypes.BlockCheckRound) {
		parent = mineBlock(t, parent, miner)
		if err := chainA.ApplyBlock(parent); err != nil {
			t.Fatal(err)
		}
		blocks = append(blocks, parent)
	}
	tip := parent

	sheet := chainA.State.GetSnapshotByHeight(tip.GetHeight())
	if sheet == nil {
		t.Fatal("chain A must hold the checkpoint sheet")
	}

	// chain B fast-syncs from genesis using the segment + sheet
	chainB := newTestChain(t)
	if err := chainB.ApplySnapshot(blocks, sheet); err != nil {
		t.Fatalf("apply snapshot: %v", err)
	}

	if h := chainB.GetLatestBlockHeight(); h != tip.GetHeight() {
		t.Fatalf("height = %d, want %d", h, tip.GetHeight())
	}
	if !bytes.Equal(chainB.GetLatestBlockHash(), tip.GetHash()) {
		t.Fatal("tip mismatch after snapshot sync")
	}
	// the trusted sheet state is queryable
	if got := balanceOf(t, chainB, miner); got.Sign() <= 0 {
		t.Fatal("the sheet balances must be applied")
	}
	// the sheet stays servable as the tip's snapshot
	if chainB.State.GetSnapshotByHeight(tip.GetHeight()) == nil {
		t.Fatal("the applied sheet must be kept as a snapshot")
	}
}

func TestApplySnapshotRejections(t *testing.T) {
	chainA := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	parent := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	blocks := make([]*ngtypes.FullBlock, 0)
	for parent.GetHeight() < uint64(ngtypes.BlockCheckRound) {
		parent = mineBlock(t, parent, miner)
		if err := chainA.ApplyBlock(parent); err != nil {
			t.Fatal(err)
		}
		blocks = append(blocks, parent)
	}
	sheet := chainA.State.GetSnapshotByHeight(parent.GetHeight())
	if sheet == nil {
		t.Fatal("missing sheet")
	}

	chainB := newTestChain(t)

	// empty segment
	if err := chainB.ApplySnapshot(nil, sheet); !errors.Is(err, ngblocks.ErrBranchEmpty) {
		t.Fatalf("empty segment: got %v, want ErrBranchEmpty", err)
	}

	// the sheet does not bind the segment tip
	badSheet := *sheet
	badSheet.Height++
	if err := chainB.ApplySnapshot(blocks, &badSheet); !errors.Is(err, blockchain.ErrSheetMismatch) {
		t.Fatalf("mismatching sheet: got %v, want ErrSheetMismatch", err)
	}

	// the segment does not attach to any stored block
	tailSheet := chainA.State.GetSnapshotByHeight(parent.GetHeight())
	if err := chainB.ApplySnapshot(blocks[1:], tailSheet); err == nil {
		t.Fatal("dangling segment must be rejected")
	}

	// nothing was applied
	if h := chainB.GetLatestBlockHeight(); h != 0 {
		t.Fatalf("height = %d, want 0", h)
	}
}
