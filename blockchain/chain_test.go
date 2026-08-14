package blockchain_test

import (
	"bytes"
	"errors"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

func newTestChain(t *testing.T) *blockchain.Chain {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "chain.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storage.InitDB(db)
	store := ngblocks.Init(db, ngtypes.ZERONET)
	state := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)

	return blockchain.Init(db, ngtypes.ZERONET, store, state)
}

// mineBlock builds and seals a valid ZERONET block on the parent, paying
// the block reward to the miner key. ZERONET's minimum difficulty is 1,
// so sealing succeeds within a few nonce attempts
func mineBlock(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey) *ngtypes.FullBlock {
	t.Helper()

	return mineBlockReward(t, parent, miner, ngtypes.GetBlockReward(parent.GetHeight()+1))
}

// mineBlockReward is mineBlock with a custom generate reward, so tests
// can craft header-valid blocks carrying invalid txs
func mineBlockReward(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey, reward *big.Int) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16

	diff := ngtypes.GetNextDiff(height, blockTime, parent)
	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(), diff)

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		reward,
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}

	if err := block.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
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

	t.Fatal("failed to seal a ZERONET block within 1e6 nonces")
	return nil
}

// mineBlockAt is mineBlockReward with an explicit timestamp, for the
// timestamp-rule tests
func mineBlockAt(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey, blockTime uint64) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, parent))

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		ngtypes.GetBlockReward(height),
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	if err := block.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
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

	t.Fatal("failed to seal")
	return nil
}

func balanceOf(t *testing.T, chain *blockchain.Chain, key *ngtypes.PrivateKey) *big.Int {
	t.Helper()

	balance, err := chain.State.GetTotalBalanceByAddress(ngtypes.NewAddress(key))
	if err != nil {
		t.Fatal(err)
	}
	return balance
}

func TestApplyBlockExtendsChain(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, miner)
	b2 := mineBlock(t, b1, miner)

	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatalf("apply b1: %v", err)
	}
	if err := chain.ApplyBlock(b2); err != nil {
		t.Fatalf("apply b2: %v", err)
	}

	if h := chain.GetLatestBlockHeight(); h != 2 {
		t.Fatalf("height = %d, want 2", h)
	}
	if !bytes.Equal(chain.GetLatestBlockHash(), b2.GetHash()) {
		t.Fatal("tip is not b2")
	}

	wantReward := new(big.Int).Add(ngtypes.GetBlockReward(1), ngtypes.GetBlockReward(2))
	if got := balanceOf(t, chain, miner); got.Cmp(wantReward) != 0 {
		t.Fatalf("miner balance = %s, want %s", got, wantReward)
	}
}

func TestApplyBlockRejectsOrphan(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, miner)
	b2 := mineBlock(t, b1, miner)

	// b2 without b1 has an unknown prev
	if err := chain.ApplyBlock(b2); err == nil {
		t.Fatal("orphan block should be rejected")
	}

	if h := chain.GetLatestBlockHeight(); h != 0 {
		t.Fatalf("height = %d, want 0", h)
	}
}

func TestReorgToHeavierBranch(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	// canonical chain: genesis <- b1 <- a2
	b1 := mineBlock(t, genesis, minerA)
	a2 := mineBlock(t, b1, minerA)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(a2); err != nil {
		t.Fatal(err)
	}

	// competing branch from b1: b1 <- b2' <- b3'
	b2 := mineBlock(t, b1, minerB)
	b3 := mineBlock(t, b2, minerB)

	// equal cumulative work: the side block is stored but no reorg happens
	if err := chain.ApplyBlock(b2); err != nil {
		t.Fatalf("side block rejected: %v", err)
	}
	if !bytes.Equal(chain.GetLatestBlockHash(), a2.GetHash()) {
		t.Fatal("tip should stay a2 on equal work")
	}
	if _, err := chain.GetBlockByHash(b2.GetHash()); err != nil {
		t.Fatal("side block should stay reachable by hash")
	}

	// the heavier branch triggers the reorg
	if err := chain.ApplyBlock(b3); err != nil {
		t.Fatalf("reorg failed: %v", err)
	}

	if h := chain.GetLatestBlockHeight(); h != 3 {
		t.Fatalf("height = %d, want 3", h)
	}
	if !bytes.Equal(chain.GetLatestBlockHash(), b3.GetHash()) {
		t.Fatal("tip should be b3 after reorg")
	}

	// the canonical height index now points at the new branch
	canon2, err := chain.GetBlockByHeight(2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canon2.GetHash(), b2.GetHash()) {
		t.Fatal("height 2 should map to b2' after reorg")
	}

	// the replaced block is still reachable by hash (as a side block now)
	if _, err := chain.GetBlockByHash(a2.GetHash()); err != nil {
		t.Fatal("replaced block a2 should stay stored by hash")
	}

	// the state was replayed onto the new branch:
	// minerA keeps only the shared b1 reward, minerB got b2'+b3'
	wantA := ngtypes.GetBlockReward(1)
	if got := balanceOf(t, chain, minerA); got.Cmp(wantA) != 0 {
		t.Fatalf("minerA balance = %s, want %s (a2 reward must be reverted)", got, wantA)
	}
	wantB := new(big.Int).Add(ngtypes.GetBlockReward(2), ngtypes.GetBlockReward(3))
	if got := balanceOf(t, chain, minerB); got.Cmp(wantB) != 0 {
		t.Fatalf("minerB balance = %s, want %s", got, wantB)
	}
}

// TestReorgRejectsInvalidBranchTxs proves the reorg is atomic AND that
// the state replay re-validates branch txs: a heavier branch carrying an
// over-reward generate tx must be rejected wholesale
func TestReorgRejectsInvalidBranchTxs(t *testing.T) {
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

	// competing branch: b2' pays itself DOUBLE the legal reward
	doubled := new(big.Int).Mul(ngtypes.GetBlockReward(2), big.NewInt(2))
	b2 := mineBlockReward(t, b1, minerB, doubled)
	b3 := mineBlock(t, b2, minerB)

	if err := chain.ApplyBlock(b2); err != nil {
		t.Fatalf("header-valid side block should be stored: %v", err)
	}

	// the heavier branch triggers the reorg, which must fail on replay
	if err := chain.ApplyBlock(b3); err == nil {
		t.Fatal("reorg onto a branch with an over-reward tx must fail")
	}

	// everything rolled back
	if !bytes.Equal(chain.GetLatestBlockHash(), a2.GetHash()) {
		t.Fatal("tip must stay a2 after the failed reorg")
	}
	if got := balanceOf(t, chain, minerB); got.Sign() != 0 {
		t.Fatalf("minerB balance = %s, want 0 after rollback", got)
	}
	wantA := new(big.Int).Add(ngtypes.GetBlockReward(1), ngtypes.GetBlockReward(2))
	if got := balanceOf(t, chain, minerA); got.Cmp(wantA) != 0 {
		t.Fatalf("minerA balance = %s, want %s after rollback", got, wantA)
	}
}

// TestReorgRespectsFinality: a gossip-driven reorg must not cross the
// last built-upon checkpoint, however heavy the branch is
func TestReorgRespectsFinality(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()

	// canonical chain to height BlockCheckRound+1: finality line sits at
	// the checkpoint (height 10)
	blocks := []*ngtypes.FullBlock{ngtypes.GetGenesisBlock(ngtypes.ZERONET)}
	for h := 0; h < int(ngtypes.BlockCheckRound)+1; h++ {
		b := mineBlock(t, blocks[len(blocks)-1], minerA)
		if err := chain.ApplyBlock(b); err != nil {
			t.Fatalf("apply block@%d: %v", b.GetHeight(), err)
		}
		blocks = append(blocks, b)
	}
	tip := blocks[len(blocks)-1]

	// a heavier branch forking BELOW the finality line (fork point 9)
	side := blocks[9] // height 9
	c10 := mineBlock(t, side, minerB)
	c11 := mineBlock(t, c10, minerB)
	c12 := mineBlock(t, c11, minerB)

	if err := chain.ApplyBlock(c10); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(c11); err != nil {
		t.Fatal(err)
	}

	err := chain.ApplyBlock(c12)
	if err == nil {
		t.Fatal("reorg below the finality line must be rejected")
	}
	if !errors.Is(err, blockchain.ErrReorgBeyondFinality) {
		t.Fatalf("got %v, want ErrReorgBeyondFinality", err)
	}

	if !bytes.Equal(chain.GetLatestBlockHash(), tip.GetHash()) {
		t.Fatal("tip must stay unchanged")
	}
}

// TestTipChangedHook: the hook fires on every tip movement (extend and
// reorg) but not on stored-only side blocks
func TestTipChangedHook(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()

	fired := 0
	chain.OnTipChanged = func() { fired++ }

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, minerA)
	a2 := mineBlock(t, b1, minerA)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(a2); err != nil {
		t.Fatal(err)
	}
	if fired != 2 {
		t.Fatalf("hook fired %d times after 2 extends, want 2", fired)
	}

	// equal-work side block: no tip movement, no hook
	b2 := mineBlock(t, b1, minerB)
	if err := chain.ApplyBlock(b2); err != nil {
		t.Fatal(err)
	}
	if fired != 2 {
		t.Fatalf("hook fired on a side block: %d", fired)
	}

	// reorg: one more fire
	b3 := mineBlock(t, b2, minerB)
	if err := chain.ApplyBlock(b3); err != nil {
		t.Fatal(err)
	}
	if fired != 3 {
		t.Fatalf("hook fired %d times after the reorg, want 3", fired)
	}
}

// TestSideBlockPruning: side blocks below the finality line get
// reclaimed at checkpoints, while canonical blocks stay
func TestSideBlockPruning(t *testing.T) {
	chain := newTestChain(t)
	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, minerA)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}

	// a competing side block at height 2
	side := mineBlock(t, b1, minerB)
	parent := mineBlock(t, b1, minerA)
	if err := chain.ApplyBlock(parent); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(side); err != nil {
		t.Fatal(err)
	}
	if _, err := chain.GetBlockByHash(side.GetHash()); err != nil {
		t.Fatal("side block should be stored")
	}

	// mine past the SECOND checkpoint: the finality line moves to 10 and
	// the old side block becomes unreachable garbage
	for parent.GetHeight() < 2*uint64(ngtypes.BlockCheckRound) {
		parent = mineBlock(t, parent, minerA)
		if err := chain.ApplyBlock(parent); err != nil {
			t.Fatalf("apply block@%d: %v", parent.GetHeight(), err)
		}
	}

	if _, err := chain.GetBlockByHash(side.GetHash()); err == nil {
		t.Fatal("finalized side block should be pruned")
	}

	// the canonical block at the same height is untouched
	canon2, err := chain.GetBlockByHeight(2)
	if err != nil {
		t.Fatal(err)
	}
	if canon2.GetHeight() != 2 {
		t.Fatal("canonical chain must stay intact")
	}
}

// TestBlockTimestampRules: non-monotonic and far-future timestamps are
// consensus-invalid (miners must not manipulate contract-visible time)
func TestBlockTimestampRules(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, miner)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}

	// equal-to-parent timestamp: rejected
	stale := mineBlockAt(t, b1, miner, b1.BlockHeader.Timestamp)
	if err := chain.ApplyBlock(stale); !errors.Is(err, blockchain.ErrBlockTimeNotMonotonic) {
		t.Fatalf("stale timestamp: got %v, want ErrBlockTimeNotMonotonic", err)
	}

	// far-future timestamp: rejected (for now). Sealed manually — the
	// miner-side CheckError loop would refuse to build it
	futureTime := uint64(time.Now().Unix()) + 3600
	future := ngtypes.NewBareBlock(ngtypes.ZERONET, 2, futureTime, b1.GetHash(),
		ngtypes.GetNextDiff(2, futureTime, b1))
	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 2,
		ngtypes.NewAddress(miner),
		ngtypes.GetBlockReward(2),
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	if err := future.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
		t.Fatal(err)
	}
	if err := future.ToSealed(utils.PackUint64LE(0)); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(future); !errors.Is(err, ngtypes.ErrBlockTimestampInvalid) {
		t.Fatalf("future timestamp: got %v, want ErrBlockTimestampInvalid", err)
	}

	// a sane timestamp extends fine
	ok := mineBlockAt(t, b1, miner, b1.BlockHeader.Timestamp+1)
	if err := chain.ApplyBlock(ok); err != nil {
		t.Fatalf("sane timestamp rejected: %v", err)
	}
}

// TestTxBlockIndex: every applied tx is locatable to its block, reorged
// txs drop out of the index and the winning branch's txs replace them
func TestTxBlockIndex(t *testing.T) {
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

	genA2 := a2.Txs[0].GetHash()
	blockHash, height, err := chain.GetTxLocation(genA2)
	if err != nil {
		t.Fatalf("locate a2's generate tx: %v", err)
	}
	if !bytes.Equal(blockHash, a2.GetHash()) || height != 2 {
		t.Fatalf("located %x@%d, want a2@2", blockHash, height)
	}

	// reorg to a heavier branch: a2's tx must leave the index
	b2 := mineBlock(t, b1, minerB)
	b3 := mineBlock(t, b2, minerB)
	if err := chain.ApplyBlock(b2); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(b3); err != nil {
		t.Fatal(err)
	}

	if _, _, err := chain.GetTxLocation(genA2); err == nil {
		t.Fatal("reorged-out tx must be unindexed")
	}

	genB2 := b2.Txs[0].GetHash()
	blockHash, height, err = chain.GetTxLocation(genB2)
	if err != nil {
		t.Fatalf("locate b2's generate tx: %v", err)
	}
	if !bytes.Equal(blockHash, b2.GetHash()) || height != 2 {
		t.Fatalf("located %x@%d, want b2'@2", blockHash, height)
	}
}

// TestSnapshotPersistence: checkpoint sheets must survive a state
// "restart" (fresh in-mem cache over the same db) so mature-balance
// lookups keep working
func TestSnapshotPersistence(t *testing.T) {
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "chain.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storage.InitDB(db)
	store := ngblocks.Init(db, ngtypes.ZERONET)
	state := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)
	chain := blockchain.Init(db, ngtypes.ZERONET, store, state)

	miner, _ := ngtypes.GenerateKey()
	parent := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	var checkpoint *ngtypes.FullBlock
	for h := 0; h < int(ngtypes.BlockCheckRound)+2; h++ {
		b := mineBlock(t, parent, miner)
		if err := chain.ApplyBlock(b); err != nil {
			t.Fatalf("apply block@%d: %v", b.GetHeight(), err)
		}
		if b.IsHead() {
			checkpoint = b
		}
		parent = b
	}

	// "restart": a fresh State over the same db has an empty mem cache
	// and must load the sheet from the snapshot bucket
	restarted := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)

	sheet := restarted.GetSnapshotByHeight(checkpoint.GetHeight())
	if sheet == nil {
		t.Fatal("persisted snapshot not found after restart")
	}
	if !bytes.Equal(sheet.BlockHash, checkpoint.GetHash()) {
		t.Fatalf("snapshot binds %x, want checkpoint %x", sheet.BlockHash, checkpoint.GetHash())
	}

	// mature balance queries work (young chain: conservative zero)
	mature, err := restarted.GetMatureBalanceByAddress(ngtypes.NewAddress(miner))
	if err != nil {
		t.Fatalf("mature balance after restart: %v", err)
	}
	if mature == nil {
		t.Fatal("mature balance must never be nil")
	}
}

func TestSwitchToBranchRejectsDetached(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, miner)
	b2 := mineBlock(t, b1, miner)

	// b1 was never applied: the branch does not attach to anything stored
	if err := chain.SwitchToBranch([]*ngtypes.FullBlock{b2}); err == nil {
		t.Fatal("detached branch should be rejected")
	}

	if h := chain.GetLatestBlockHeight(); h != 0 {
		t.Fatalf("height = %d, want 0", h)
	}
}
