// Package e2e boots multiple full in-process nodes (storage + chain +
// state + real libp2p networking + consensus) and verifies that forks
// get resolved across the network.
package e2e

import (
	"bytes"
	"context"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/c0mm4nd/wasman/wat"
	"github.com/libp2p/go-libp2p/core/peer"
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
	db    *bbolt.DB

	stopped bool
}

// newNode boots a full node on an ephemeral tcp port with its own db
// mustWat compiles wat authoring text into the wasm binary the chain
// stores — wat is a test-only convenience, the chain runs wasm
func mustWat(source string) []byte {
	bin, err := wat.Compile([]byte(source))
	if err != nil {
		panic("test wat does not compile: " + err.Error())
	}
	return bin
}

func newNode(t *testing.T) *testNode {
	t.Helper()

	return newNodeAt(t, t.TempDir())
}

// newNodeAt boots (or re-boots) a full node from the given data dir, so
// restart tests can resume from a persisted chain
func newNodeAt(t *testing.T, dir string) *testNode {
	t.Helper()

	return bootNode(t, dir, false)
}

// newSnapshotNode boots a node running in snapshot (fast-sync) mode
func newSnapshotNode(t *testing.T) *testNode {
	t.Helper()

	return bootNode(t, t.TempDir(), true)
}

func bootNode(t *testing.T, dir string, snapshotMode bool) *testNode {
	t.Helper()

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
		MinPeers:                    1,
		ReconnectInterval:           time.Second,
	})
	local.GoServe()

	pool := ngpool.Init(db, chain, local)
	pool.MinFeePerByte = nil // tests deal in raw units; the floor has unit coverage

	pow := consensus.InitPoWConsensus(db, chain, pool, state, local, consensus.PoWorkConfig{
		Network:                     ngtypes.ZERONET,
		SnapshotMode:                snapshotMode,
		DisableConnectingBootstraps: true,
	})
	pow.GoLoop()

	node := &testNode{pow: pow, chain: chain, local: local, db: db}

	// a stopped node must not touch the db anymore, so closing it right
	// after is safe — this is the shutdown api's e2e coverage
	t.Cleanup(node.shutdown)

	return node
}

// shutdown stops the node and closes its db; safe to call twice
func (n *testNode) shutdown() {
	if n.stopped {
		return
	}
	n.stopped = true

	n.pow.Stop()
	time.Sleep(100 * time.Millisecond) // let the loops drain
	_ = n.db.Close()
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
func mineOn(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16

	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, parent))
	block.SetCoinbase(ngtypes.NewAddress(miner))

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

	t.Fatal("failed to seal a ZERONET block")
	return nil
}

// mineAndSubmit mines on the node's current tip and submits through the
// full MinedNewBlock path (import + p2p broadcast)
func mineAndSubmit(t *testing.T, node *testNode, miner *ngtypes.PrivateKey) *ngtypes.FullBlock {
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

func balanceOf(t *testing.T, node *testNode, key *ngtypes.PrivateKey) *big.Int {
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

	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()

	// shared prefix mined by A, propagated to B over pubsub
	shared := mineAndSubmit(t, nodeA, minerA)
	waitTip(t, nodeB, shared.GetHash(), 10*time.Second)

	// A extends the canonical chain; B follows over the network
	a2 := mineAndSubmit(t, nodeA, minerA)
	waitTip(t, nodeB, a2.GetHash(), 10*time.Second)

	// B mines a competing branch on the shared block. Force its hash HIGHER
	// than a2's, so the deterministic equal-work tie-break keeps a2 (a
	// lower-hash competitor would correctly win the tie — covered elsewhere).
	var b2 *ngtypes.FullBlock
	for {
		minerB, _ = ngtypes.GenerateKey()
		b2 = mineOn(t, shared, minerB)
		if bytes.Compare(b2.GetHash(), a2.GetHash()) > 0 {
			break
		}
	}
	if err := nodeB.pow.MinedNewBlock(b2); err != nil {
		t.Fatalf("submit fork block: %v", err)
	}

	// equal work: both nodes keep a2 (a2 has the smaller hash)
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
func mineOnTxs(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey, txs ...*ngtypes.FullTx) *ngtypes.FullBlock {
	t.Helper()
	return mineOnAll(t, parent, miner, nil, txs...)
}

// mineOnAll is mineOn with extra txs AND blind commitments packed in. The
// commitments must be set before ToUnsealing, which folds x.Commits into the
// content root alongside the txs.
func mineOnAll(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey, commits []*ngtypes.Commitment, txs ...*ngtypes.FullTx) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + height*16

	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, parent))
	block.SetCoinbase(ngtypes.NewAddress(miner))

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		ngtypes.GetBlockReward(height),
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	// SetCommits must precede ToUnsealing: the content root covers both
	if len(commits) != 0 {
		block.SetCommits(commits)
	}
	if err := block.ToUnsealing(append([]*ngtypes.FullTx{genTx}, txs...)); err != nil {
		t.Fatal(err)
	}
	// ToUnsealing computes the witness root over the txs in insertion order but
	// stores them hash-sorted (NewTxTrie sorts in place); when a packed tx sorts
	// ahead of the generate tx the two orders diverge and CheckError's
	// CalcWitnessRoot(x.Txs) (over the sorted txs) rejects the block. Recompute
	// the witness root over the canonical sorted order before sealing so it is
	// folded into the pow preimage consistently — exactly what peers re-validate.
	block.BlockHeader.WitnessRoot = ngtypes.CalcWitnessRoot(block.Txs, block.Commits)

	var lastErr error
	for n := uint64(0); n < 1_000_000; n++ {
		if err := block.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if lastErr = block.CheckError(); lastErr == nil {
			return block
		}
	}

	t.Fatalf("failed to seal a ZERONET block: %v", lastErr)
	return nil
}

// e2eSalt is the reveal nonce every effect tx in these tests carries.
var e2eSalt = []byte("e2e-mempool-salt-0123456789")

// revealTx sets the private-mempool Salt on an effect tx and signs it, turning
// it into a valid reveal (its commitment must already be on chain).
func revealTx(t *testing.T, tx *ngtypes.FullTx, key *ngtypes.PrivateKey) *ngtypes.FullTx {
	t.Helper()

	tx.Salt = e2eSalt
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}
	return tx
}

// commitFor builds and signs the blind commitment for a reveal tx, to be
// packed at commitHeight (which must be strictly earlier than the tx's own
// reveal height). The commitment hash is blake3(tx.UnheightedHash() ‖ Salt).
func commitFor(t *testing.T, tx *ngtypes.FullTx, key *ngtypes.PrivateKey, commitHeight uint64) *ngtypes.Commitment {
	t.Helper()

	hash := utils.Hash256(append(tx.UnheightedHash(), tx.Salt...))
	commit := ngtypes.NewCommitment(ngtypes.ZERONET, commitHeight, hash, big.NewInt(0))
	if err := commit.Signature(key); err != nil {
		t.Fatal(err)
	}
	return commit
}

// TestTxPropagation covers the full tx lifecycle across the network:
// a tx submitted on one node reaches the other's pool over pubsub, gets
// mined there, and the resulting block settles the state on both sides
func TestTxPropagation(t *testing.T) {
	nodeA := newNode(t)
	nodeB := newNode(t)
	connect(t, nodeA, nodeB)

	key, _ := ngtypes.GenerateKey()

	// fund the key (via node A): the address spends directly
	b1 := mineAndSubmit(t, nodeA, key)
	waitTip(t, nodeB, b1.GetHash(), 10*time.Second)
	b2 := mineAndSubmit(t, nodeA, key)
	waitTip(t, nodeB, b2.GetHash(), 10*time.Second)

	// build the transact reveal (locked to its reveal height 4) and its blind
	// commitment, packed one block earlier at height 3
	var dest ngtypes.Address
	dest[0] = 0xee
	tx := revealTx(t, ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 4,
		dest, big.NewInt(10), big.NewInt(1), nil, nil), key)
	commit := commitFor(t, tx, key, 3)

	// mine the commit-carrying block on A; both nodes record it at height 3,
	// strictly before the reveal
	b3 := mineOnAll(t, b2, key, []*ngtypes.Commitment{commit})
	if err := nodeA.pow.MinedNewBlock(b3); err != nil {
		t.Fatalf("submit commit block on nodeA: %v", err)
	}
	waitTip(t, nodeB, b3.GetHash(), 10*time.Second)

	// now the reveal is pool-admissible: submit it on A, it must reach B's pool
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
	pack := nodeB.pow.Pool.GetPack(4)
	if len(pack) != 1 {
		t.Fatalf("nodeB pack size = %d, want 1", len(pack))
	}
	b4 := mineOnTxs(t, b3, key, pack...)
	if err := nodeB.pow.MinedNewBlock(b4); err != nil {
		t.Fatalf("mine tx block on nodeB: %v", err)
	}
	waitTip(t, nodeA, b4.GetHash(), 10*time.Second)

	for name, node := range map[string]*testNode{"A": nodeA, "B": nodeB} {
		got, err := node.chain.State.GetTotalBalanceByAddress(dest)
		if err != nil {
			t.Fatal(err)
		}
		if got.Int64() != 10 {
			t.Fatalf("node%s: dest balance = %s, want 10", name, got)
		}
		// the mined block moved the tip: both pools must be empty now
		if len(node.pow.Pool.GetPack(5)) != 0 {
			t.Fatalf("node%s: pool must reset on the tip change", name)
		}
	}
}

// TestRestartPersistence: a node stopped cleanly must come back from
// its data dir with the chain and state intact, stay live (mine more)
// and serve peers again
func TestRestartPersistence(t *testing.T) {
	dir := t.TempDir()
	miner, _ := ngtypes.GenerateKey()

	// first life: mine three blocks onto the persistent db
	node := newNodeAt(t, dir)
	var tip *ngtypes.FullBlock
	for i := 0; i < 3; i++ {
		tip = mineAndSubmit(t, node, miner)
	}
	wantBalance := balanceOf(t, node, miner)
	node.shutdown()

	// second life: same data dir
	node = newNodeAt(t, dir)

	if h := node.chain.GetLatestBlockHeight(); h != 3 {
		t.Fatalf("restarted height = %d, want 3", h)
	}
	if !bytes.Equal(node.chain.GetLatestBlockHash(), tip.GetHash()) {
		t.Fatal("restarted tip mismatch")
	}
	if got := balanceOf(t, node, miner); got.Cmp(wantBalance) != 0 {
		t.Fatalf("restarted balance = %s, want %s", got, wantBalance)
	}

	// still live: extends its own chain past the checkpoint round...
	var newTip *ngtypes.FullBlock
	for node.chain.GetLatestBlockHeight() < uint64(ngtypes.BlockCheckRound)+2 {
		newTip = mineAndSubmit(t, node, miner)
	}

	// ...and serves a fresh peer over the wired sync protocol (the peer
	// is behind a full round, so its sync loop fetches the whole chain)
	peer := newNode(t)
	connect(t, peer, node)
	waitTip(t, peer, newTip.GetHash(), 45*time.Second)
}

// TestOrphanOutOfOrderImport: gossip blocks arriving before their
// parents get parked and cascade in as soon as the gap closes
func TestOrphanOutOfOrderImport(t *testing.T) {
	node := newNode(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineOn(t, genesis, miner)
	b2 := mineOn(t, b1, miner)
	b3 := mineOn(t, b2, miner)

	// newest-first delivery: b3 and b2 must park, not error
	if err := node.pow.ImportBlock(b3); err != nil {
		t.Fatalf("orphan b3 should park: %v", err)
	}
	if err := node.pow.ImportBlock(b2); err != nil {
		t.Fatalf("orphan b2 should park: %v", err)
	}
	if h := node.chain.GetLatestBlockHeight(); h != 0 {
		t.Fatalf("height = %d before the gap closes, want 0", h)
	}

	// the missing parent lands: the whole burst cascades in
	if err := node.pow.ImportBlock(b1); err != nil {
		t.Fatal(err)
	}

	if h := node.chain.GetLatestBlockHeight(); h != 3 {
		t.Fatalf("height = %d after cascade, want 3", h)
	}
	if !bytes.Equal(node.chain.GetLatestBlockHash(), b3.GetHash()) {
		t.Fatal("tip should be b3")
	}
}

// TestPeerReconnect: after a connection drop the peer manager must
// redial known peers and restore the link without any manual action
func TestPeerReconnect(t *testing.T) {
	nodeA := newNode(t)
	nodeB := newNode(t)
	connect(t, nodeA, nodeB)

	if len(nodeA.local.Network().Peers()) != 1 {
		t.Fatal("setup: nodeA should have one peer")
	}

	// force-drop the connection
	if err := nodeA.local.Network().ClosePeer(nodeB.local.ID()); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if len(nodeA.local.Network().Peers()) >= 1 {
			// the link is back; make sure it actually works end to end
			miner, _ := ngtypes.GenerateKey()
			time.Sleep(time.Second) // let pubsub re-mesh
			b := mineAndSubmit(t, nodeA, miner)
			waitTip(t, nodeB, b.GetHash(), 10*time.Second)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatal("peer manager never restored the dropped connection")
}

// TestContractLifecycle drives a wat contract through its whole
// on-chain life across two nodes: deploy (commit tx) -> activate (lock
// tx) -> trigger (transact tx runs the vm) — and both nodes must agree
// on the contract's kv state written by the vm
func TestContractLifecycle(t *testing.T) {
	const contractWat = `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "ng:main")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))))
`

	nodeA := newNode(t)
	nodeB := newNode(t)
	connect(t, nodeA, nodeB)

	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	// mineNext mines a block on A's current tip carrying the given commits and
	// txs, and waits for B to converge onto it
	mineNext := func(commits []*ngtypes.Commitment, txs ...*ngtypes.FullTx) *ngtypes.FullBlock {
		tip := nodeA.chain.GetLatestBlock().(*ngtypes.FullBlock)
		b := mineOnAll(t, tip, key, commits, txs...)
		if err := nodeA.pow.MinedNewBlock(b); err != nil {
			t.Fatalf("submit block@%d: %v", b.GetHeight(), err)
		}
		waitTip(t, nodeB, b.GetHash(), 10*time.Second)
		return b
	}

	// commitReveal builds an effect tx locked to the NEXT-NEXT height, packs its
	// blind commitment in the next block, then reveals & executes it in the block
	// after — the mandatory commit-reveal split
	commitReveal := func(build func(revealHeight uint64) *ngtypes.FullTx) *ngtypes.FullBlock {
		commitHeight := nodeA.chain.GetLatestBlockHeight() + 1
		revealHeight := commitHeight + 1
		tx := revealTx(t, build(revealHeight), key)
		commit := commitFor(t, tx, key, commitHeight)
		mineNext([]*ngtypes.Commitment{commit})
		return mineNext(nil, tx)
	}

	// fund the deployer: two block rewards cover the deploy fee + change
	mineNext(nil)
	mineNext(nil)

	// deploy: opens the address's slot and goes live at once (compiles the
	// module and enables the vm)
	rawExtra := ngtypes.EncodeCommitCode(mustWat(contractWat))
	commitReveal(func(h uint64) *ngtypes.FullTx {
		return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.DeployTx, h,
			ngtypes.Address{}, nil, big.NewInt(1), rawExtra, nil)
	})

	// trigger: a transact tx to the contract account runs `main`
	commitReveal(func(h uint64) *ngtypes.FullTx {
		return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, h,
			addr, big.NewInt(1), big.NewInt(1), nil, nil)
	})

	// both nodes hold the identical contract state written by the vm
	for name, node := range map[string]*testNode{"A": nodeA, "B": nodeB} {
		acc, err := node.chain.State.GetContract(addr)
		if err != nil {
			t.Fatalf("node%s: %v", name, err)
		}
		if !bytes.Equal(acc.Source, mustWat(contractWat)) {
			t.Fatalf("node%s: contract text mismatch", name)
		}
		if !acc.IsActive() {
			t.Fatalf("node%s: contract should be locked", name)
		}
		if got := string(acc.Context.Get("key")); got != "val" {
			t.Fatalf("node%s: contract kv = %q, want \"val\"", name, got)
		}
	}
}

// TestSnapshotSync: a fresh node in snapshot mode fast-syncs to the
// serving node's checkpoint by fetching the chain segment plus the
// checkpoint state sheet, applied atomically (no tx replay)
func TestSnapshotSync(t *testing.T) {
	if testing.Short() {
		t.Skip("snapshot sync needs the 10s sync loop")
	}

	server := newNode(t)
	miner, _ := ngtypes.GenerateKey()

	// the server mines past its first checkpoint, which makes a servable
	// state snapshot at height BlockCheckRound
	var checkpoint *ngtypes.FullBlock
	for i := 0; i < int(ngtypes.BlockCheckRound)+2; i++ {
		b := mineAndSubmit(t, server, miner)
		if b.GetHeight() == uint64(ngtypes.BlockCheckRound) {
			checkpoint = b
		}
	}

	client := newSnapshotNode(t)
	connect(t, client, server)

	// the snapshot sync lands the client exactly on the checkpoint
	waitTip(t, client, checkpoint.GetHash(), 45*time.Second)

	// the client state came from the sheet: the miner holds the rewards
	// of blocks 1..checkpoint
	want := big.NewInt(0)
	for h := uint64(1); h <= uint64(ngtypes.BlockCheckRound); h++ {
		want.Add(want, ngtypes.GetBlockReward(h))
	}
	if got := balanceOf(t, client, miner); got.Cmp(want) != 0 {
		t.Fatalf("client balance = %s, want %s (from the sheet)", got, want)
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

	minerA, _ := ngtypes.GenerateKey()
	minerB, _ := ngtypes.GenerateKey()

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

// TestAllTxVerbsViaNetwork drives every tx verb through the FULL
// network path — submitted into node A's pool, gossiped to node B,
// mined there and settled on both — asserting the state effects and
// the exact fee burns of the whole lifecycle:
//
//	Generate -> Transact(pay) -> Deploy(init, live) ->
//	Transact(main+value) -> Transact(selector) -> Deploy(upgrade) ->
//	Transact(upgraded main) -> Destroy
func TestAllTxVerbsViaNetwork(t *testing.T) {
	// data layout: boot@0(4) hit@4(3) main@7(4) ping@11(4) paid@15(4) args@19(4)
	const srcV1 = `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (import "tx" "get_extra" (func $args (param i32) (result i32)))
  (import "tx" "get_paid" (func $paid (param i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "boothitmainpingpaidargs")
  (func (export "ng:init")
    (drop (call $set (i32.const 0) (i32.const 4) (i32.const 0) (i32.const 4))))
  ;; the UUPS upgrade hook: its mere presence authorizes a re-deploy
  (func (export "ng:upgrade"))
  (func (export "ng:main")
    (drop (call $set (i32.const 4) (i32.const 3) (i32.const 7) (i32.const 4)))
    (drop (call $set (i32.const 15) (i32.const 4) (i32.const 64) (call $paid (i32.const 64)))))
  (func (export "ping")
    (drop (call $set (i32.const 4) (i32.const 3) (i32.const 11) (i32.const 4)))
    (drop (call $set (i32.const 19) (i32.const 4) (i32.const 64) (call $args (i32.const 64))))))
`
	// srcV1 records "main" at hit; srcV3 is the upgraded module recording
	// "M3IN" instead, still exporting upgrade so it too stays upgradeable
	srcV3 := strings.Replace(srcV1, "boothitmainping", "boothitM3INping", 1)

	nodeA := newNode(t)
	nodeB := newNode(t)
	connect(t, nodeA, nodeB)

	deployer, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(deployer)
	minerB, _ := ngtypes.GenerateKey()

	// Generate: the deployer mines its own funds on node A
	for i := 0; i < 3; i++ {
		b := mineAndSubmit(t, nodeA, deployer)
		waitTip(t, nodeB, b.GetHash(), 10*time.Second)
	}

	// relay pushes a signed tx through A's pool, waits for the gossip
	// to reach B, lets B mine it and both nodes converge. The first tx
	// carries the full envelope (registering the key); every later one
	// uses the compact form, saving the public key bytes
	keyOnChain := false
	relay := func(build func(height uint64) *ngtypes.FullTx) *ngtypes.FullTx {
		t.Helper()

		// mandatory commit-reveal: the effect tx reveals one block after its
		// blind commitment. commitHeight is the next block; the reveal is locked
		// one higher (strictly later, the anti-same-block-reaction rule).
		commitHeight := nodeA.chain.GetLatestBlockHeight() + 1
		revealHeight := commitHeight + 1
		tx := revealTx(t, build(revealHeight), deployer)
		if keyOnChain {
			// revealTx already full-signed it; re-sign compact once the key is
			// registered on chain, re-carrying the same Salt
			tx.Salt = e2eSalt
			if err := tx.SignatureCompact(deployer); err != nil {
				t.Fatal(err)
			}
			wantLen := 2 + ngtypes.AddressSize + ngtypes.SigSize(deployer.Scheme)
			if ngtypes.HasRecovery(deployer.Scheme) {
				wantLen = 2 + ngtypes.SigSize(deployer.Scheme)
			}
			if len(tx.Sign) != wantLen {
				t.Fatalf("compact envelope size = %d, want %d", len(tx.Sign), wantLen)
			}
		} else {
			keyOnChain = true
		}

		// first, land the blind commitment: B mines it at commitHeight and A
		// converges, so the reveal is admissible into A's pool
		commit := commitFor(t, tx, deployer, commitHeight)
		cTip := nodeB.chain.GetLatestBlock().(*ngtypes.FullBlock)
		cb := mineOnAll(t, cTip, minerB, []*ngtypes.Commitment{commit})
		if err := nodeB.pow.MinedNewBlock(cb); err != nil {
			t.Fatalf("mine commit block for tx type %d: %v", tx.Type, err)
		}
		waitTip(t, nodeA, cb.GetHash(), 10*time.Second)

		if err := nodeA.pow.Pool.PutNewTxFromLocal(tx); err != nil {
			t.Fatalf("submit %d tx: %v", tx.Type, err)
		}

		deadline := time.Now().Add(10 * time.Second)
		for {
			if exists, _ := nodeB.pow.Pool.IsInPool(tx.GetHash()); exists {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("tx type %d never reached nodeB's pool", tx.Type)
			}
			time.Sleep(100 * time.Millisecond)
		}

		pack := nodeB.pow.Pool.GetPack(revealHeight)
		if len(pack) != 1 {
			t.Fatalf("nodeB pack size = %d, want 1", len(pack))
		}
		tip := nodeB.chain.GetLatestBlock().(*ngtypes.FullBlock)
		b := mineOnTxs(t, tip, minerB, pack...)
		if err := nodeB.pow.MinedNewBlock(b); err != nil {
			t.Fatalf("mine tx type %d: %v", tx.Type, err)
		}
		waitTip(t, nodeA, b.GetHash(), 10*time.Second)

		// the tip moved on nodeA, but OnTipChanged→Pool.Reset runs just after
		// the tip becomes visible; wait for the mined tx to actually leave
		// nodeA's pool before the next relay reuses the same (fee-0) sender,
		// otherwise PutNewTxFromLocal races a not-yet-reset pool
		poolCleared := time.Now().Add(5 * time.Second)
		for {
			if exists, _ := nodeA.pow.Pool.IsInPool(tx.GetHash()); !exists {
				break
			}
			if time.Now().After(poolCleared) {
				t.Fatalf("nodeA pool never cleared the mined tx type %d", tx.Type)
			}
			time.Sleep(50 * time.Millisecond)
		}
		return tx
	}

	bothNodes := func(check func(name string, node *testNode)) {
		t.Helper()
		for name, node := range map[string]*testNode{"A": nodeA, "B": nodeB} {
			check(name, node)
		}
	}

	// Transact: a plain payment to a fresh address
	var payee ngtypes.Address
	payee[0] = 0xe1
	oneNG := new(big.Int).Set(ngtypes.NG)
	relay(func(h uint64) *ngtypes.FullTx {
		return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, h,
			payee, oneNG, nil, nil, nil)
	})
	bothNodes(func(name string, node *testNode) {
		if got, _ := node.chain.State.GetTotalBalanceByAddress(payee); got.Cmp(oneNG) != 0 {
			t.Fatalf("node%s: payee balance = %s, want %s", name, got, oneNG)
		}
	})

	// Deploy: opens the namespace and goes LIVE at once, running init —
	// no separate activate step
	deployExtra := ngtypes.EncodeCommitCode(mustWat(srcV1))
	relay(func(h uint64) *ngtypes.FullTx {
		return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.DeployTx, h,
			ngtypes.Address{}, nil, nil, deployExtra, nil)
	})
	bothNodes(func(name string, node *testNode) {
		c, err := node.chain.State.GetContract(addr)
		if err != nil {
			t.Fatalf("node%s: deploy did not open the slot: %v", name, err)
		}
		if !bytes.Equal(c.Source, mustWat(srcV1)) || !c.IsActive() {
			t.Fatalf("node%s: contract should be live after deploy", name)
		}
		if got := string(c.Context.Get("boot")); got != "boot" {
			t.Fatalf("node%s: init did not run, boot=%q", name, got)
		}
	})

	// Transact (fallback): value + empty extra runs main, which
	// records the marker and msg.value
	twoNG := new(big.Int).Mul(ngtypes.NG, big.NewInt(2))
	relay(func(h uint64) *ngtypes.FullTx {
		return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, h,
			addr, twoNG, nil, nil, nil)
	})
	bothNodes(func(name string, node *testNode) {
		c, _ := node.chain.State.GetContract(addr)
		if got := string(c.Context.Get("hit")); got != "main" {
			t.Fatalf("node%s: main hit = %q, want main", name, got)
		}
		// get_paid writes a fixed 32-byte little-endian amount (the u256
		// ABI money format); the contract stored all 32 bytes at "paid"
		got := c.Context.Get("paid")
		be := make([]byte, len(got))
		for i, b := range got {
			be[len(got)-1-i] = b
		}
		if len(got) != 32 || new(big.Int).SetBytes(be).Cmp(twoNG) != 0 {
			t.Fatalf("node%s: paid = %x, want 32-byte LE of %x", name, got, twoNG.Bytes())
		}
	})

	// Transact (selector): the eth-style selector routes to ping
	selTx := relay(func(h uint64) *ngtypes.FullTx {
		return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, h,
			addr, nil, nil, ngtypes.EncodeCallData("ping", []byte("xy")), nil)
	})
	bothNodes(func(name string, node *testNode) {
		c, _ := node.chain.State.GetContract(addr)
		if got := string(c.Context.Get("hit")); got != "ping" {
			t.Fatalf("node%s: selector hit = %q, want ping", name, got)
		}
		if got := string(c.Context.Get("args")); got != "xy" {
			t.Fatalf("node%s: selector args = %q, want xy", name, got)
		}
		runs, err := node.chain.State.GetTxRuns(selTx.GetHash())
		if err != nil || len(runs) != 1 || runs[0].Entry != "ping" || !runs[0].Ok {
			t.Fatalf("node%s: receipt runs = %+v (%v)", name, runs, err)
		}
	})

	// Deploy (upgrade): a second deploy replaces the code UUPS-style — the
	// live contract's `upgrade` hook authorizes the swap, and the module
	// stays live with init re-run as a migration
	upgradeExtra := ngtypes.EncodeCommitCode(mustWat(srcV3))
	relay(func(h uint64) *ngtypes.FullTx {
		return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.DeployTx, h,
			ngtypes.Address{}, nil, nil, upgradeExtra, nil)
	})
	bothNodes(func(name string, node *testNode) {
		c, _ := node.chain.State.GetContract(addr)
		if !bytes.Equal(c.Source, mustWat(srcV3)) || !c.IsActive() {
			t.Fatalf("node%s: upgrade did not replace the live code", name)
		}
	})

	// Transact after the upgrade: main now records the upgraded marker
	relay(func(h uint64) *ngtypes.FullTx {
		return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, h,
			addr, nil, nil, nil, nil)
	})
	bothNodes(func(name string, node *testNode) {
		c, _ := node.chain.State.GetContract(addr)
		if got := string(c.Context.Get("hit")); got != "M3IN" {
			t.Fatalf("node%s: upgraded main hit = %q, want M3IN", name, got)
		}
	})

	// Destroy: an empty-code deploy on the live slot, authorized by the
	// contract's own `upgrade` hook — the slot (source AND context) disappears
	destroyExtra := ngtypes.EncodeCommitCode(nil)
	relay(func(h uint64) *ngtypes.FullTx {
		return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.DeployTx, h,
			ngtypes.Address{}, nil, nil, destroyExtra, nil)
	})
	bothNodes(func(name string, node *testNode) {
		if _, err := node.chain.State.GetContract(addr); err == nil {
			t.Fatalf("node%s: the slot must be gone after destroy", name)
		}
	})

	// the exact fee ledger: 3 rewards in, 1 NG paid away; the 2 NG to
	// the own contract was a self-transfer, and every tx fee was zero
	want := new(big.Int).Mul(ngtypes.GetBlockReward(1), big.NewInt(3))
	want.Sub(want, oneNG)
	bothNodes(func(name string, node *testNode) {
		got, _ := node.chain.State.GetTotalBalanceByAddress(addr)
		if got.Cmp(want) != 0 {
			t.Fatalf("node%s: deployer balance = %s, want %s", name, got, want)
		}
	})
}
