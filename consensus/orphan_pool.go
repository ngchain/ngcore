package consensus

import (
	"encoding/hex"
	"sync"

	"github.com/ngchain/ngcore/ngtypes"
)

// maxOrphanBlocks caps the steady-state orphan buffer; at fast block
// times a small window is enough to absorb gossip reordering
const maxOrphanBlocks = 128

// orphanPool buffers gossip blocks which arrived before their parent
// (out-of-order delivery), keyed by the missing parent hash. It is NOT
// used by the initial sync, which fetches ordered ranges
type orphanPool struct {
	sync.Mutex

	byPrev map[string][]*ngtypes.FullBlock
	count  int
}

func newOrphanPool() *orphanPool {
	return &orphanPool{
		byPrev: make(map[string][]*ngtypes.FullBlock),
	}
}

// add stashes the orphan; duplicates and overflow are dropped
func (op *orphanPool) add(block *ngtypes.FullBlock) bool {
	op.Lock()
	defer op.Unlock()

	if op.count >= maxOrphanBlocks {
		return false
	}

	key := hex.EncodeToString(block.GetPrevHash())
	hash := block.GetHash()
	for _, waiting := range op.byPrev[key] {
		if string(waiting.GetHash()) == string(hash) {
			return false // duplicate
		}
	}

	op.byPrev[key] = append(op.byPrev[key], block)
	op.count++

	return true
}

// take pops all orphans waiting for the given parent hash
func (op *orphanPool) take(parentHash []byte) []*ngtypes.FullBlock {
	op.Lock()
	defer op.Unlock()

	key := hex.EncodeToString(parentHash)
	children := op.byPrev[key]
	if children != nil {
		delete(op.byPrev, key)
		op.count -= len(children)
	}

	return children
}
