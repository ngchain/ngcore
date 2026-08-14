// Package e2e boots multiple full in-process nodes (storage + chain +
// state + real libp2p networking + consensus) and verifies that forks
// get resolved across the network.
package e2e

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/ngchain/secp256k1"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

type testNode struct {
	pow   *consensus.PoWork
	chain *blockchain.Chain
	local *ngp2p.LocalNode
}

// newNode boots a full node on an ephemeral tcp port with its own db
func newNode(t *testing.T) *testNode {
	t.Helper()

	dir := t.TempDir()

	db, err := bbolt.Open(filepath.Join(dir, "chain.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}

	storage.InitDB(db)
	store := ngblocks.Init(db, ngtypes.ZERONET)
	state := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)
	chain := blockchain.Init(db, ngtypes.ZERONET, store, state)

	local := ngp2p.InitLocalNode(chain, ngp2p.P2PConfig{
		P2PKeyFile:                  filepath.Join(dir, "p2p.key"),
		Network:                     ngtypes.ZERONET,
		Port:                        0, // ephemeral
		DisableDiscovery:            true,
		DisableConnectingBootstraps: true,
	})
	local.GoServe()

	pool := ngpool.Init(db, chain, local)

	pow := consensus.InitPoWConsensus(db, chain, pool, state, local, consensus.PoWorkConfig{
		Network:                     ngtypes.ZERONET,
		DisableConnectingBootstraps: true,
	})
	pow.GoLoop()

	// a stopped node must not touch the db anymore, so closing it right
	// after is safe — this is the shutdown api's e2e coverage
	t.Cleanup(func() {
		pow.Stop()
		time.Sleep(100 * time.Millisecond) // let the loops drain
		_ = db.Close()
	})

	return &testNode{pow: pow, chain: chain, local: local}
}

// connect dials b from a and waits until the pubsub meshes know each other
func connect(t *testing.T, a, b *testNode) {
	t.Helper()

	err := a.local.Connect(context.Background(), peer.AddrInfo{
		ID:    b.local.ID(),
		Addrs: b.local.Addrs(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// floodsub learns the peer's subscriptions right after the connect
	// handshake; give it a moment
	time.Sleep(time.Second)
}

// mineOn seals a valid ZERONET block on the given parent (instant: the
// regression network's difficulty is 1)
func mineOn(t *testing.T, parent *ngtypes.FullBlock, miner *secp256k1.PrivateKey) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16

	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, parent))

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height, 0,
		[]ngtypes.Address{ngtypes.NewAddress(miner)},
		[]*big.Int{ngtypes.GetBlockReward(height)},
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

	t.Fatal("failed to seal a ZERONET block")
	return nil
}

// mineAndSubmit mines on the node's current tip and submits through the
// full MinedNewBlock path (import + p2p broadcast)
func mineAndSubmit(t *testing.T, node *testNode, miner *secp256k1.PrivateKey) *ngtypes.FullBlock {
	t.Helper()

	tip := node.chain.GetLatestBlock().(*ngtypes.FullBlock)
	block := mineOn(t, tip, miner)
	if err := node.pow.MinedNewBlock(block); err != nil {
		t.Fatalf("submit mined block@%d: %v", block.GetHeight(), err)
	}
	return block
}

// waitTip polls until the node's canonical tip is the wanted hash
func waitTip(t *testing.T, node *testNode, want []byte, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if bytes.Equal(node.chain.GetLatestBlockHash(), want) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("tip %x@%d never became %x", node.chain.GetLatestBlockHash(),
		node.chain.GetLatestBlockHeight(), want)
}

func balanceOf(t *testing.T, node *testNode, key *secp256k1.PrivateKey) *big.Int {
	t.Helper()

	balance, err := node.chain.State.GetTotalBalanceByAddress(ngtypes.NewAddress(key))
	if err != nil {
		t.Fatal(err)
	}
	return balance
}

// TestForkResolutionViaBroadcast covers the gossip path: two connected
// nodes fork at the same height, and the heavier branch wins on BOTH —
// one node reorgs via local fork choice, the other via p2p broadcast
func TestForkResolutionViaBroadcast(t *testing.T) {
	nodeA := newNode(t)
	nodeB := newNode(t)
	connect(t, nodeA, nodeB)

	minerA, _ := secp256k1.GeneratePrivateKey()
	minerB, _ := secp256k1.GeneratePrivateKey()

	// shared prefix mined by A, propagated to B over pubsub
	shared := mineAndSubmit(t, nodeA, minerA)
	waitTip(t, nodeB, shared.GetHash(), 10*time.Second)

	// A extends the canonical chain; B follows over the network
	a2 := mineAndSubmit(t, nodeA, minerA)
	waitTip(t, nodeB, a2.GetHash(), 10*time.Second)

	// B mines a competing branch on the shared block
	b2 := mineOn(t, shared, minerB)
	if err := nodeB.pow.MinedNewBlock(b2); err != nil {
		t.Fatalf("submit fork block: %v", err)
	}

	// equal work: both nodes keep a2
	time.Sleep(2 * time.Second)
	if !bytes.Equal(nodeA.chain.GetLatestBlockHash(), a2.GetHash()) {
		t.Fatal("nodeA should keep a2 on equal work")
	}
	if !bytes.Equal(nodeB.chain.GetLatestBlockHash(), a2.GetHash()) {
		t.Fatal("nodeB should keep a2 on equal work")
	}

	// the extra block makes B's branch heavier: B reorgs locally and A
	// follows through the broadcast
	b3 := mineOn(t, b2, minerB)
	if err := nodeB.pow.MinedNewBlock(b3); err != nil {
		t.Fatalf("submit reorg block: %v", err)
	}

	waitTip(t, nodeB, b3.GetHash(), 10*time.Second)
	waitTip(t, nodeA, b3.GetHash(), 10*time.Second)

	// both nodes agree on the replayed state
	for name, node := range map[string]*testNode{"A": &*nodeA, "B": &*nodeB} {
		if h := node.chain.GetLatestBlockHeight(); h != 3 {
			t.Fatalf("node%s height = %d, want 3", name, h)
		}

		wantA := ngtypes.GetBlockReward(1) // a2's reward must be reverted
		if got := balanceOf(t, node, minerA); got.Cmp(wantA) != 0 {
			t.Fatalf("node%s: minerA balance = %s, want %s", name, got, wantA)
		}

		wantB := new(big.Int).Add(ngtypes.GetBlockReward(2), ngtypes.GetBlockReward(3))
		if got := balanceOf(t, node, minerB); got.Cmp(wantB) != 0 {
			t.Fatalf("node%s: minerB balance = %s, want %s", name, got, wantB)
		}
	}
}

// mineOnTxs is mineOn with extra (non-generate) txs packed in
func mineOnTxs(t *testing.T, parent *ngtypes.FullBlock, miner *secp256k1.PrivateKey, txs ...*ngtypes.FullTx) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16

	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, parent))

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height, 0,
		[]ngtypes.Address{ngtypes.NewAddress(miner)},
		[]*big.Int{ngtypes.GetBlockReward(height)},
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	if err := block.ToUnsealing(append([]*ngtypes.FullTx{genTx}, txs...)); err != nil {
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

	t.Fatal("failed to seal a ZERONET block")
	return nil
}

// TestTxPropagation covers the full tx lifecycle across the network:
// a tx submitted on one node reaches the other's pool over pubsub, gets
// mined there, and the resulting block settles the state on both sides
func TestTxPropagation(t *testing.T) {
	nodeA := newNode(t)
	nodeB := newNode(t)
	connect(t, nodeA, nodeB)

	key, _ := secp256k1.GeneratePrivateKey()

	// fund the key and register account 700 for it (via node A)
	b1 := mineAndSubmit(t, nodeA, key)
	waitTip(t, nodeB, b1.GetHash(), 10*time.Second)

	extra := make([]byte, 8)
	binary.LittleEndian.PutUint64(extra, 700)
	regTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.RegisterTx, 2, 1,
		[]ngtypes.Address{ngtypes.NewAddress(key)},
		[]*big.Int{big.NewInt(0)},
		ngtypes.RegisterFee, extra, nil)
	if err := regTx.Signature(key); err != nil {
		t.Fatal(err)
	}
	b2 := mineOnTxs(t, b1, key, regTx)
	if err := nodeA.pow.MinedNewBlock(b2); err != nil {
		t.Fatalf("submit register block: %v", err)
	}
	waitTip(t, nodeB, b2.GetHash(), 10*time.Second)

	// submit a transact tx on A: it must reach B's pool over the network
	var dest ngtypes.Address
	dest[0] = 0xee
	tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 3, 700,
		[]ngtypes.Address{dest}, []*big.Int{big.NewInt(10)}, big.NewInt(1), nil, nil)
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}
	if err := nodeA.pow.Pool.PutNewTxFromLocal(tx); err != nil {
		t.Fatalf("submit tx on nodeA: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		if exists, _ := nodeB.pow.Pool.IsInPool(tx.GetHash()); exists {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("tx never reached nodeB's pool")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// B mines the pool pack; the block settles the tx on both nodes
	pack := nodeB.pow.Pool.GetPack(3)
	if len(pack) != 1 {
		t.Fatalf("nodeB pack size = %d, want 1", len(pack))
	}
	b3 := mineOnTxs(t, b2, key, pack...)
	if err := nodeB.pow.MinedNewBlock(b3); err != nil {
		t.Fatalf("mine tx block on nodeB: %v", err)
	}
	waitTip(t, nodeA, b3.GetHash(), 10*time.Second)

	for name, node := range map[string]*testNode{"A": nodeA, "B": nodeB} {
		got, err := node.chain.State.GetTotalBalanceByAddress(dest)
		if err != nil {
			t.Fatal(err)
		}
		if got.Int64() != 10 {
			t.Fatalf("node%s: dest balance = %s, want 10", name, got)
		}
		// the mined block moved the tip: both pools must be empty now
		if len(node.pow.Pool.GetPack(3)) != 0 {
			t.Fatalf("node%s: pool must reset on the tip change", name)
		}
	}
}

// TestDeepForkConvergeViaSync covers the sync-module path: two nodes
// build long conflicting chains in isolation; after connecting, the
// lighter node must converge onto the heavier chain through the wired
// sync protocol (checkpoint compare -> fetch -> atomic switch)
func TestDeepForkConvergeViaSync(t *testing.T) {
	if testing.Short() {
		t.Skip("deep converge needs the 10s sync loop")
	}

	nodeA := newNode(t)
	nodeB := newNode(t)

	minerA, _ := secp256k1.GeneratePrivateKey()
	minerB, _ := secp256k1.GeneratePrivateKey()

	// isolated: A mines 3 blocks, B mines a full checkpoint round + 2
	for i := 0; i < 3; i++ {
		mineAndSubmit(t, nodeA, minerA)
	}
	var bTip *ngtypes.FullBlock
	for i := 0; i < int(ngtypes.BlockCheckRound)+2; i++ {
		bTip = mineAndSubmit(t, nodeB, minerB)
	}

	if nodeA.chain.GetLatestBlockHeight() != 3 {
		t.Fatal("nodeA setup failed")
	}
	if nodeB.chain.GetLatestBlockHeight() != uint64(ngtypes.BlockCheckRound)+2 {
		t.Fatal("nodeB setup failed")
	}

	connect(t, nodeA, nodeB)

	// the sync loop ticks every 10s: status exchange, failed sync, then
	// converging — give it a few rounds
	waitTip(t, nodeA, bTip.GetHash(), 90*time.Second)

	if got := balanceOf(t, nodeA, minerA); got.Sign() != 0 {
		t.Fatalf("minerA balance = %s, want 0 (whole A-chain reverted)", got)
	}
}
