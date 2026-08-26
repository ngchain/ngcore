package ngpool_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// builtBlocks indexes test-built blocks by their sealed hash so sealStateRoot
// can walk a block's ancestry and reproduce its committed post-state root (now
// part of the pow preimage). Register a block only after ToSealed.
var builtBlocks = map[string]*ngtypes.FullBlock{}

// sealStateRoot sets an unsealing block's post-state StateRoot by replaying its
// ancestry (genesis then the registered parent chain) into a throwaway state
// and dry-applying — reveals resolve against commitments recorded by replayed
// ancestors. Call AFTER ToUnsealing and BEFORE ToSealed.
func sealStateRoot(t *testing.T, block *ngtypes.FullBlock) {
	t.Helper()
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	var ancestry []*ngtypes.FullBlock
	prev := block.GetPrevHash()
	for !bytes.Equal(prev, genesis.GetHash()) {
		p, ok := builtBlocks[string(prev)]
		if !ok {
			t.Fatalf("sealStateRoot: ancestor %x not registered", prev)
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
		t.Fatalf("sealStateRoot: ancestry replay: %v", err)
	}

	root, err := ngstate.DryApplyRoot(scratch, block)
	if err != nil {
		t.Fatalf("sealStateRoot: dry apply: %v", err)
	}
	block.BlockHeader.StateRoot = root
}

func registerBuilt(b *ngtypes.FullBlock) { builtBlocks[string(b.GetHash())] = b }
