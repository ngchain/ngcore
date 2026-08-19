package consensus

import (
	"sync"

	"github.com/ngchain/ngcore/ngtypes"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/utils"
)

const (
	// minDesiredPeerCount gates the dangerous sync ops (converging) on
	// public networks; the p2p peer manager also targets this count
	minDesiredPeerCount = 3
)

// syncModule is a submodule to the pow, managing the sync of blocks.
type syncModule struct {
	pow *PoWork

	localNode *ngp2p.LocalNode

	storeMu sync.RWMutex
	store   map[peer.ID]*RemoteRecord

	*utils.Locker
}

// newSyncModule creates a new sync module.
func newSyncModule(c ngtypes.Consensus, localNode *ngp2p.LocalNode) *syncModule {
	pow := c.(*PoWork)
	syncMod := &syncModule{
		pow:       pow,
		localNode: localNode,
		storeMu:   sync.RWMutex{},
		store:     make(map[peer.ID]*RemoteRecord),

		Locker: utils.NewLocker(),
	}

	latest := pow.Chain.GetLatestBlock()
	log.Warnf("current latest block: %x@%d", latest.GetHash(), latest.GetHeight())

	return syncMod
}

// put the peer and its remote status into mod.
func (mod *syncModule) putRemote(id peer.ID, remote *RemoteRecord) {
	mod.storeMu.Lock()
	defer mod.storeMu.Unlock()
	mod.store[id] = remote
}

// upsertRemote records or refreshes a peer's status under ONE write lock.
// The callers used to read store[id] then update/put unlocked, racing
// putRemote's write (storeMu guarded the write but not those reads)
func (mod *syncModule) upsertRemote(id peer.ID, origin, latest uint64, checkpointHash, checkpointActualDiff []byte) {
	mod.storeMu.Lock()
	defer mod.storeMu.Unlock()
	if r, exists := mod.store[id]; exists {
		r.update(origin, latest, checkpointHash, checkpointActualDiff)
	} else {
		mod.store[id] = NewRemoteRecord(id, origin, latest, checkpointHash, checkpointActualDiff)
	}
}

// remotesSnapshot copies the current records under a read lock so the map
// is never iterated concurrently with a write
func (mod *syncModule) remotesSnapshot() []*RemoteRecord {
	mod.storeMu.RLock()
	defer mod.storeMu.RUnlock()
	slice := make([]*RemoteRecord, 0, len(mod.store))
	for _, v := range mod.store {
		slice = append(slice, v)
	}
	return slice
}
