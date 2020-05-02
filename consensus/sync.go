package consensus

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
)

type syncModule struct {
	sync.RWMutex
	store *sync.Map
}

type remoteRecord struct {
	id peer.ID
	origin uint64
	latest uint64
	checkpointHash []byte
}

func newSyncModule() *syncModule {
	return &syncModule{
		store: &sync.Map{},
	}
}

func (sync *syncModule) PutRemote(id peer.ID, remote *remoteRecord) {
	sync.store.Store(id, remote)
}

func (sync *syncModule) GetRemote(id peer.ID)  *remoteRecord {
	i, exists := sync.store.Load(id)
	if !exists {
		return nil
	}

	return i.(*remoteRecord)
}
