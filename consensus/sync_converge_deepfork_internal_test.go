package consensus

import (
	"bytes"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// deepForkBuilt indexes test-built blocks by their sealed hash so sealAtTime
// can walk a block's ancestry and reproduce its committed StateRoot.
var deepForkBuilt = map[string]*ngtypes.FullBlock{}

// sealAtTime builds+seals a ZERONET block on parent with an explicit
// (backdated) timestamp, so a test can build a chain without tripping the 15s
// future-drift wall that fast GetBlockTemplate mining hits. It also seals the
// post-state StateRoot into the header (now part of the pow preimage) by
// replaying the block's ancestry into a throwaway state.
func sealAtTime(t *testing.T, parent *ngtypes.FullBlock, key *ngtypes.PrivateKey, blockTime uint64) *ngtypes.FullBlock {
	t.Helper()
	h := parent.GetHeight() + 1
	b := ngtypes.NewBareBlock(ngtypes.ZERONET, h, blockTime, parent.GetHash(),
		ngtypes.GetNextDiff(h, blockTime, parent))
	b.SetCoinbase(ngtypes.NewAddress(key))
	gen := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, h,
		ngtypes.NewAddress(key), ngtypes.GetBlockReward(h), big.NewInt(0), nil, nil)
	if err := gen.Signature(key); err != nil {
		t.Fatal(err)
	}
	if err := b.ToUnsealing([]*ngtypes.FullTx{gen}); err != nil {
		t.Fatal(err)
	}
	b.BlockHeader.StateRoot = deepForkStateRoot(t, b)

	for n := uint64(0); n < 2_000_000; n++ {
		if err := b.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if b.CheckError() == nil {
			deepForkBuilt[string(b.GetHash())] = b
			return b
		}
	}
	t.Fatal("failed to seal")
	return nil
}

// deepForkStateRoot replays the block's ancestry (genesis then the registered
// parent chain) into a throwaway state and returns the root block produces.
func deepForkStateRoot(t *testing.T, block *ngtypes.FullBlock) []byte {
	t.Helper()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	var ancestry []*ngtypes.FullBlock
	prev := block.GetPrevHash()
	for !bytes.Equal(prev, genesis.GetHash()) {
		p, ok := deepForkBuilt[string(prev)]
		if !ok {
			t.Fatalf("sealAtTime: ancestor %x not registered", prev)
		}
		ancestry = append([]*ngtypes.FullBlock{p}, ancestry...)
		prev = p.GetPrevHash()
	}

	sdb, err := bbolt.Open(filepath.Join(t.TempDir(), "scratch-root.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sdb.Close() }()
	storage.InitDB(sdb)
	scratch := ngstate.InitStateFromGenesis(sdb, ngtypes.ZERONET)

	if err := scratch.Update(func(txn *bbolt.Tx) error {
		for _, b := range ancestry {
			if err := scratch.Upgrade(txn, b); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("sealAtTime: ancestry replay: %v", err)
	}

	root, err := ngstate.DryApplyRoot(scratch, block)
	if err != nil {
		t.Fatalf("sealAtTime: dry apply: %v", err)
	}
	return root
}

// TestDoConvergingDeepFork reproduces (and fixes) the production failure: a
// node whose fork suffix is deeper than one converging batch could never
// converge back to the heavier chain. With convergeBatchSize=2 and a 6-block
// fork suffix, the pre-fix code returned a non-attaching segment and stalled;
// the fix walks the batches back to the real fork point and returns the
// attaching divergent branch.
func TestDoConvergingDeepFork(t *testing.T) {
	base := uint64(time.Now().UnixMilli()) - 2000

	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})

	// shared prefix (heights 1..3): the same blocks on both nodes
	sharedKey, _ := ngtypes.GenerateKey()
	parent := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	for i := 1; i <= 3; i++ {
		b := sealAtTime(t, parent, sharedKey, base+uint64(i))
		if err := remote.Chain.ApplyBlock(b); err != nil {
			t.Fatalf("remote shared@%d: %s", b.GetHeight(), err)
		}
		if err := local.Chain.ApplyBlock(b); err != nil {
			t.Fatalf("local shared@%d: %s", b.GetHeight(), err)
		}
		parent = b
	}
	forkPoint := parent // height 3

	// remote's heavier fork: heights 4..11
	rKey, _ := ngtypes.GenerateKey()
	rParent := forkPoint
	for i := 4; i <= 11; i++ {
		b := sealAtTime(t, rParent, rKey, base+uint64(i))
		if err := remote.Chain.ApplyBlock(b); err != nil {
			t.Fatalf("remote fork@%d: %s", b.GetHeight(), err)
		}
		rParent = b
	}

	// local's own fork suffix: heights 4..9 (6 blocks, > batch size)
	lKey, _ := ngtypes.GenerateKey()
	lParent := forkPoint
	for i := 4; i <= 9; i++ {
		b := sealAtTime(t, lParent, lKey, base+uint64(i)+100) // +100 so hashes differ
		if err := local.Chain.ApplyBlock(b); err != nil {
			t.Fatalf("local fork@%d: %s", b.GetHeight(), err)
		}
		lParent = b
	}

	local.SyncMod.convergeBatchSize = 2 // force a multi-batch walk over the fork
	connectNodes(t, local, remote)
	mod := local.SyncMod
	rec := recordFor(remote)

	var lastErr error
	for i := 0; i < 20; i++ {
		if string(local.Chain.GetLatestBlockHash()) == string(remote.Chain.GetLatestBlockHash()) {
			break
		}
		if err := mod.doSync(rec); err != nil {
			lastErr = err
			if cerr := mod.doConverging(rec); cerr != nil {
				lastErr = cerr
			}
		}
	}

	if string(local.Chain.GetLatestBlockHash()) != string(remote.Chain.GetLatestBlockHash()) {
		t.Fatalf("deep fork did not converge: local=%d remote=%d lastErr=%v",
			local.Chain.GetLatestBlockHeight(), remote.Chain.GetLatestBlockHeight(), lastErr)
	}
	if h := local.Chain.GetLatestBlockHeight(); h != 11 {
		t.Fatalf("converged height = %d, want 11", h)
	}
}
