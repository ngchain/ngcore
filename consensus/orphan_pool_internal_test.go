package consensus

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

// fakeOrphan builds a distinct sealed-enough block for the pool (only
// the header hashes matter to the orphan pool)
func fakeOrphan(height uint64, prev byte) *ngtypes.FullBlock {
	return ngtypes.NewBareBlock(ngtypes.ZERONET, height, uint64(time.Now().Unix()),
		bytes.Repeat([]byte{prev}, 32), big.NewInt(1))
}

func TestOrphanPool(t *testing.T) {
	op := newOrphanPool()

	b1 := fakeOrphan(1, 0x01)
	b2 := fakeOrphan(2, 0x01) // same missing parent as b1
	b3 := fakeOrphan(3, 0x02)

	if !op.add(b1) || !op.add(b2) || !op.add(b3) {
		t.Fatal("fresh orphans must be accepted")
	}
	if op.count != 3 {
		t.Fatalf("count = %d, want 3", op.count)
	}

	// duplicates are dropped
	if op.add(b1) {
		t.Fatal("a duplicate orphan must be rejected")
	}
	if op.count != 3 {
		t.Fatalf("count after the duplicate = %d, want 3", op.count)
	}

	// take pops the whole sibling group at once
	got := op.take(bytes.Repeat([]byte{0x01}, 32))
	if len(got) != 2 {
		t.Fatalf("take popped %d orphans, want 2", len(got))
	}
	if op.count != 1 {
		t.Fatalf("count after take = %d, want 1", op.count)
	}

	// a second take for the same parent finds nothing
	if again := op.take(bytes.Repeat([]byte{0x01}, 32)); again != nil {
		t.Fatalf("second take returned %d orphans, want none", len(again))
	}

	// re-adding after take works (no stale duplicate marker)
	if !op.add(b1) {
		t.Fatal("an orphan must be addable again after being taken")
	}
}

func TestOrphanPoolOverflow(t *testing.T) {
	op := newOrphanPool()

	op.count = maxOrphanBlocks
	if op.add(fakeOrphan(1, 0x03)) {
		t.Fatal("a full pool must drop new orphans")
	}

	op.count = maxOrphanBlocks - 1
	if !op.add(fakeOrphan(1, 0x04)) {
		t.Fatal("the last free slot must still accept an orphan")
	}
	if op.add(fakeOrphan(2, 0x05)) {
		t.Fatal("the pool must be full now")
	}
}
