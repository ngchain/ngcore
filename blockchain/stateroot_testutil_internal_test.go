package blockchain

import (
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// sealInternalStateRoot sets an unsealing block's post-state StateRoot against
// its own ancestry (walked from chain's block store into a throwaway state),
// so the sealed block passes the apply-time CheckStateRoot. The header now
// commits to this root in the pow preimage; a side block's root is relative to
// its own fork point, which the ancestry replay reproduces. Call AFTER
// ToUnsealing and BEFORE ToSealed.
func sealInternalStateRoot(t *testing.T, chain *Chain, block *ngtypes.FullBlock) {
	t.Helper()

	var ancestry []*ngtypes.FullBlock
	cur, err := chain.GetBlockByHash(block.GetPrevHash())
	if err != nil {
		t.Fatalf("sealInternalStateRoot: parent %x not in store: %v", block.GetPrevHash(), err)
	}
	for {
		fb := cur.(*ngtypes.FullBlock)
		ancestry = append([]*ngtypes.FullBlock{fb}, ancestry...)
		if fb.IsGenesis() {
			break
		}
		cur, err = chain.GetBlockByHash(fb.GetPrevHash())
		if err != nil {
			t.Fatalf("sealInternalStateRoot: ancestry break at %x: %v", fb.GetPrevHash(), err)
		}
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
			if b.IsGenesis() {
				continue
			}
			if err := scratch.Upgrade(txn, b); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("sealInternalStateRoot: ancestry replay: %v", err)
	}

	root, err := ngstate.DryApplyRoot(scratch, block)
	if err != nil {
		t.Fatalf("sealInternalStateRoot: dry apply: %v", err)
	}
	block.BlockHeader.StateRoot = root
}
