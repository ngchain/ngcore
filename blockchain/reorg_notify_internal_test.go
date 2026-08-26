package blockchain

import (
	"math/big"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

func newInternalTestChain(t *testing.T) *Chain {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "chain.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storage.InitDB(db)
	store := ngblocks.Init(db, ngtypes.ZERONET)
	state := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)
	return Init(db, ngtypes.ZERONET, store, state)
}

func sealChildBlock(t *testing.T, chain *Chain, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := parent.GetTimestamp() + 16
	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(),
		ngtypes.GetNextDiff(height, blockTime, parent))
	block.SetCoinbase(ngtypes.NewAddress(miner))

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner), ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	if err := block.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
		t.Fatal(err)
	}
	sealInternalStateRoot(t, chain, block)
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

// TestApplyBlockDropsStaleReorgLogs guards the reorg-notify lifecycle: a reorg
// txn sets reorgRemoved inside the write txn, so if that txn aborts the logs
// linger. A later fast-path block must NOT fire them as removed — ApplyBlock
// resets reorgRemoved before its txn, so notifyReorg only ever emits logs the
// current call committed.
func TestApplyBlockDropsStaleReorgLogs(t *testing.T) {
	chain := newInternalTestChain(t)

	fired := false
	chain.OnReorg = func(removed, added []ngstate.Log) { fired = true }

	// simulate the residue of a reorg txn that gathered logs then aborted
	chain.reorgRemoved = []ngstate.Log{{Height: 1, Event: ngstate.Event{Topic: "stale"}}}
	chain.reorgAdded = []ngstate.Log{{Height: 1, Event: ngstate.Event{Topic: "stale"}}}

	miner, _ := ngtypes.GenerateKey()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := sealChildBlock(t, chain, genesis, miner)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatalf("apply b1: %v", err)
	}

	if fired {
		t.Fatal("fast-path block fired OnReorg with stale orphaned logs")
	}
	if chain.reorgRemoved != nil || chain.reorgAdded != nil {
		t.Fatalf("reorg log buffers not cleared: removed=%+v added=%+v", chain.reorgRemoved, chain.reorgAdded)
	}
}
