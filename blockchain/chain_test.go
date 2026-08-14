package blockchain_test

import (
	"bytes"
	"errors"
	"math/big"
	"path/filepath"
	"testing"

	"github.com/ngchain/secp256k1"
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
func mineBlock(t *testing.T, parent *ngtypes.FullBlock, miner *secp256k1.PrivateKey) *ngtypes.FullBlock {
	t.Helper()

	return mineBlockReward(t, parent, miner, ngtypes.GetBlockReward(parent.GetHeight()+1))
}

// mineBlockReward is mineBlock with a custom generate reward, so tests
// can craft header-valid blocks carrying invalid txs
func mineBlockReward(t *testing.T, parent *ngtypes.FullBlock, miner *secp256k1.PrivateKey, reward *big.Int) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16

	diff := ngtypes.GetNextDiff(height, blockTime, parent)
	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(), diff)

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height, 0,
		[]ngtypes.Address{ngtypes.NewAddress(miner)},
		[]*big.Int{reward},
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

func balanceOf(t *testing.T, chain *blockchain.Chain, key *secp256k1.PrivateKey) *big.Int {
	t.Helper()

	balance, err := chain.State.GetTotalBalanceByAddress(ngtypes.NewAddress(key))
	if err != nil {
		t.Fatal(err)
	}
	return balance
}

func TestApplyBlockExtendsChain(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := secp256k1.GeneratePrivateKey()

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
	miner, _ := secp256k1.GeneratePrivateKey()

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
	minerA, _ := secp256k1.GeneratePrivateKey()
	minerB, _ := secp256k1.GeneratePrivateKey()

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
	minerA, _ := secp256k1.GeneratePrivateKey()
	minerB, _ := secp256k1.GeneratePrivateKey()

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
	minerA, _ := secp256k1.GeneratePrivateKey()
	minerB, _ := secp256k1.GeneratePrivateKey()

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
	minerA, _ := secp256k1.GeneratePrivateKey()
	minerB, _ := secp256k1.GeneratePrivateKey()

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

func TestSwitchToBranchRejectsDetached(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := secp256k1.GeneratePrivateKey()

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
