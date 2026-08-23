package consensus

import (
	"bytes"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
)

func TestTemplateBlockTime(t *testing.T) {
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	// the genesis timestamp lies in the past, so the template takes
	// the wall clock
	now := uint64(time.Now().UnixMilli())
	got := templateBlockTime(genesis)
	if got <= genesis.BlockHeader.Timestamp {
		t.Fatalf("templateBlockTime = %d, not after the parent %d", got, genesis.BlockHeader.Timestamp)
	}
	if got < now || got > now+2000 {
		t.Fatalf("templateBlockTime = %d, want the wall clock ~%d", got, now)
	}

	// a parent already ahead of the wall clock forces parent+1
	future := uint64(time.Now().UnixMilli()) + 100
	parent := ngtypes.NewBareBlock(ngtypes.ZERONET, 1, future, genesis.GetHash(), big.NewInt(1))
	if got := templateBlockTime(parent); got != future+1 {
		t.Fatalf("templateBlockTime = %d, want parent+1 = %d", got, future+1)
	}
}

func TestCreateGenerateTx(t *testing.T) {
	key, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	extra := []byte("miner-extra")
	tx := CreateGenerateTx(ngtypes.ZERONET, key, 7, extra)

	if tx.Type != ngtypes.GenerateTx {
		t.Fatalf("tx type = %d, want GenerateTx", tx.Type)
	}
	if tx.Height != 7 {
		t.Fatalf("tx height = %d, want 7", tx.Height)
	}
	if !tx.To.Equals(ngtypes.NewAddress(key)) {
		t.Fatalf("tx receiver = %s, want the miner address", tx.To)
	}
	if tx.Value.Cmp(ngtypes.GetBlockReward(7)) != 0 {
		t.Fatalf("tx value = %s, want the block reward %s", tx.Value, ngtypes.GetBlockReward(7))
	}
	if !bytes.Equal(tx.Extra, extra) {
		t.Fatalf("tx extra = %x, want %x", tx.Extra, extra)
	}
	if !tx.IsSigned() {
		t.Fatal("generate tx is not signed")
	}
}

func TestGetBlockTemplate(t *testing.T) {
	extra := []byte("template-extra")
	pow := newTestNode(t, PoWorkConfig{
		DisableConnectingBootstraps: true,
		MinerExtraData:              extra,
	})
	key, _ := ngtypes.GenerateKey()

	genesisHash := pow.Chain.GetLatestBlockHash()
	tmpl := pow.GetBlockTemplate(key).(*ngtypes.FullBlock)

	if tmpl.GetHeight() != 1 {
		t.Fatalf("template height = %d, want 1", tmpl.GetHeight())
	}
	if !bytes.Equal(tmpl.GetPrevHash(), genesisHash) {
		t.Fatalf("template prev hash = %x, want the genesis %x", tmpl.GetPrevHash(), genesisHash)
	}
	if !tmpl.IsUnsealing() {
		t.Fatal("template must carry its txs (unsealing state)")
	}
	if len(tmpl.Txs) == 0 || tmpl.Txs[0].Type != ngtypes.GenerateTx {
		t.Fatal("template must open with the generate tx")
	}
	if !bytes.Equal(tmpl.Txs[0].Extra, extra) {
		t.Fatalf("generate tx extra = %q, want the configured MinerExtraData %q", tmpl.Txs[0].Extra, extra)
	}

	// the template must be sealable and land on the chain
	sealBlock(t, tmpl)
	if err := pow.Chain.ApplyBlock(tmpl); err != nil {
		t.Fatalf("sealed template rejected by the chain: %s", err)
	}
}

func TestGetBareBlockTemplateWithTxs(t *testing.T) {
	pow := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})

	bare, txs := pow.GetBareBlockTemplateWithTxs()

	if bare.GetHeight() != 1 {
		t.Fatalf("bare template height = %d, want 1", bare.GetHeight())
	}
	if !bytes.Equal(bare.GetPrevHash(), pow.Chain.GetLatestBlockHash()) {
		t.Fatal("bare template must build on the local tip")
	}
	if len(bare.Txs) != 0 {
		t.Fatal("bare template must not embed any tx")
	}
	if len(txs) != 0 {
		t.Fatalf("tx pack = %d txs, want none on a fresh pool", len(txs))
	}
}

func TestGetChain(t *testing.T) {
	pow := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})

	if pow.GetChain() != pow.Chain {
		t.Fatal("GetChain must expose the underlying chain")
	}
}

func TestMinedNewBlock(t *testing.T) {
	pow := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()

	// while the sync module is busy, mined blocks must be refused
	pow.SyncMod.Locker.Lock()
	someBlock := sealBlock(t, pow.GetBlockTemplate(key).(*ngtypes.FullBlock))
	if err := pow.MinedNewBlock(someBlock); !errors.Is(err, ErrChainOnSyncing) {
		t.Fatalf("MinedNewBlock while syncing = %v, want ErrChainOnSyncing", err)
	}
	pow.SyncMod.Locker.Unlock()

	// the happy path applies and broadcasts
	if err := pow.MinedNewBlock(someBlock); err != nil {
		t.Fatalf("MinedNewBlock failed: %s", err)
	}
	if pow.Chain.GetLatestBlockHeight() != 1 {
		t.Fatalf("height after MinedNewBlock = %d, want 1", pow.Chain.GetLatestBlockHeight())
	}

	// a block detached from every stored block must be rejected
	orphan := ngtypes.NewBareBlock(ngtypes.ZERONET, 5, uint64(time.Now().UnixMilli()),
		bytes.Repeat([]byte{0xaa}, 32), big.NewInt(1))
	if err := orphan.ToUnsealing([]*ngtypes.FullTx{CreateGenerateTx(ngtypes.ZERONET, key, 5, nil)}); err != nil {
		t.Fatal(err)
	}
	sealBlock(t, orphan)
	if err := pow.MinedNewBlock(orphan); !errors.Is(err, ngblocks.ErrPrevBlockNotExist) {
		t.Fatalf("MinedNewBlock with unknown prev = %v, want ErrPrevBlockNotExist", err)
	}
}

// TestImportBlockOrphanCascade replays a gossip reordering: children
// arrive before their parent, get parked, and cascade in once the
// parent lands; an invalid parked block is dropped without breaking
// the cascade
func TestImportBlockOrphanCascade(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	chain := mineBlocks(t, remote, key, 3)
	a1, a2, a3 := chain[0], chain[1], chain[2]

	pow := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})

	// while the sync module is busy, a gossip block is PARKED (not dropped),
	// so the node isn't isolated from blocks arriving mid-sync
	pow.SyncMod.Locker.Lock()
	if err := pow.ImportBlock(a1); err != nil {
		t.Fatalf("ImportBlock while syncing should park (nil), got: %v", err)
	}
	pow.SyncMod.Locker.Unlock()
	if pow.Chain.GetLatestBlockHeight() != 0 {
		t.Fatal("a block parked during sync must not move the tip")
	}

	// out-of-order arrival: a2 and a3 get parked
	if err := pow.ImportBlock(a2); err != nil {
		t.Fatalf("importing a2 before a1 should park it, got: %s", err)
	}
	if err := pow.ImportBlock(a3); err != nil {
		t.Fatalf("importing a3 before a1 should park it, got: %s", err)
	}
	if pow.Chain.GetLatestBlockHeight() != 0 {
		t.Fatal("parked blocks must not move the tip")
	}

	// a parked block that turns out invalid (height gap) is skipped
	bad := ngtypes.NewBareBlock(ngtypes.ZERONET, 99, uint64(time.Now().UnixMilli()), a2.GetHash(), big.NewInt(1))
	if err := bad.ToUnsealing([]*ngtypes.FullTx{CreateGenerateTx(ngtypes.ZERONET, key, 99, nil)}); err != nil {
		t.Fatal(err)
	}
	sealBlock(t, bad)
	if err := pow.ImportBlock(bad); err != nil {
		t.Fatalf("importing the bad orphan should park it, got: %s", err)
	}

	// the parent unlocks the whole burst
	if err := pow.ImportBlock(a1); err != nil {
		t.Fatalf("importing a1 failed: %s", err)
	}
	if h := pow.Chain.GetLatestBlockHeight(); h != 3 {
		t.Fatalf("height after the cascade = %d, want 3", h)
	}
	if !bytes.Equal(pow.Chain.GetLatestBlockHash(), a3.GetHash()) {
		t.Fatal("tip after the cascade is not a3")
	}

	// a non-orphan invalid block surfaces its error
	detached := ngtypes.NewBareBlock(ngtypes.ZERONET, 99, uint64(time.Now().UnixMilli()),
		pow.Chain.GetOriginBlock().GetHash(), big.NewInt(1))
	if err := detached.ToUnsealing([]*ngtypes.FullTx{CreateGenerateTx(ngtypes.ZERONET, key, 99, nil)}); err != nil {
		t.Fatal(err)
	}
	sealBlock(t, detached)
	if err := pow.ImportBlock(detached); err == nil {
		t.Fatal("a known-parent block with a broken height must be rejected")
	}

	// a full orphan pool refuses to park and surfaces the orphan error
	pow.orphans.count = maxOrphanBlocks
	unknownParent := ngtypes.NewBareBlock(ngtypes.ZERONET, 7, uint64(time.Now().UnixMilli()),
		bytes.Repeat([]byte{0xbb}, 32), big.NewInt(1))
	if err := unknownParent.ToUnsealing([]*ngtypes.FullTx{CreateGenerateTx(ngtypes.ZERONET, key, 7, nil)}); err != nil {
		t.Fatal(err)
	}
	sealBlock(t, unknownParent)
	if err := pow.ImportBlock(unknownParent); !errors.Is(err, ngblocks.ErrPrevBlockNotExist) {
		t.Fatalf("ImportBlock on a full orphan pool = %v, want ErrPrevBlockNotExist", err)
	}
	pow.orphans.count = 0
}

// TestGoLoopEventLoop feeds the p2p broadcast channels which the event
// loop consumes: a valid block lands on the chain, an invalid tx only
// logs a warning
func TestGoLoopEventLoop(t *testing.T) {
	pow := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()

	pow.GoLoop()

	block := sealBlock(t, pow.GetBlockTemplate(key).(*ngtypes.FullBlock))
	pow.LocalNode.OnBlock <- block

	waitUntil(t, 5*time.Second, func() bool {
		return pow.Chain.GetLatestBlockHeight() == 1
	}, "the event loop never imported the broadcast block")

	// a tx from an unfunded address cannot enter the pool: the failure
	// branch only warns
	badTx := ngtypes.NewUnsignedTx(ngtypes.ZERONET, ngtypes.TransactTx, 2,
		ngtypes.NewAddress(key), big.NewInt(1), big.NewInt(0), nil)
	if err := badTx.Signature(key); err != nil {
		t.Fatal(err)
	}
	pow.LocalNode.OnTx <- badTx

	time.Sleep(200 * time.Millisecond)
	if pow.Chain.GetLatestBlockHeight() != 1 {
		t.Fatal("the invalid tx must not affect the chain")
	}
}
