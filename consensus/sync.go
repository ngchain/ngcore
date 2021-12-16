package consensus

import (
	"github.com/ngchain/ngcore/ngtypes"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/utils"
)

const (
	minDesiredPeerCount = 3 // TODO: add peer num requirement, avoid mining alone
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
