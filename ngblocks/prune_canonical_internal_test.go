package ngblocks

import (
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestPruneKeepsCanonicalPromotedFromSide reproduces the DB corruption behind
// the CheckHealth restart panic: a block first seen as a side block, then
// promoted to canonical via the fast path (PutNewBlock), kept its stale
// sideBlockKey, so PruneSideBlocks deleted its body while the height index
// still pointed at it.
func TestPruneKeepsCanonicalPromotedFromSide(t *testing.T) {
	db := newDB(t)
	_ = Init(db, ngtypes.ZERONET)

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	miner := newKey(t)
	b1 := buildBlock(t, genesis, miner) // height 1

	// 1) b1 first arrives out of order and is stored as a SIDE block
	update(t, db, func(bb, _ *bbolt.Bucket) error { return PutSideBlock(bb, b1) })

	// 2) later b1 becomes canonical via the fast path
	update(t, db, func(bb, tb *bbolt.Bucket) error { return PutNewBlock(bb, tb, b1) })

	// 3) side-block pruning runs at a checkpoint
	update(t, db, func(bb, _ *bbolt.Bucket) error {
		_, err := PruneSideBlocks(bb, 1000)
		return err
	})

	// 4) the canonical block@1 must survive (CheckHealth walks this)
	update(t, db, func(bb, _ *bbolt.Bucket) error {
		if _, err := GetBlockByHeight(bb, 1); err != nil {
			t.Fatalf("canonical block@1 lost after prune: %v", err)
		}
		return nil
	})
}
