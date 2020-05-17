package consensus

import (
	"bytes"
	"sort"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/storage"
)

type syncModule struct {
	sync.RWMutex
	*PoWork

	store map[peer.ID]*remoteRecord
}

type remoteRecord struct {
	id             peer.ID
	origin         uint64 // rank 1
	latest         uint64
	checkpointHash []byte
	lastChatTime   int64
}

// checkpoint fork rule: when a node mined a checkpoint, all other node are forced to start sync
func (r *remoteRecord) shouldSync() bool {
	if !bytes.Equal(r.checkpointHash, storage.GetChain().GetLatestCheckpointHash()) &&
		r.latest > storage.GetChain().GetLatestBlockHeight() {
		return true
	}

	return false
}

func newSyncModule(pow *PoWork, isBootstrapNode bool) *syncModule {
	sync := &syncModule{
		RWMutex: sync.RWMutex{},
		PoWork:  pow,
		store:   make(map[peer.ID]*remoteRecord),
	}

	if !isBootstrapNode {
		sync.bootstrap()
	}

	return sync
}

func (sync *syncModule) PutRemote(id peer.ID, remote *remoteRecord) {
	sync.Lock()
	sync.store[id] = remote
	sync.Unlock()
}

func (sync *syncModule) GetRemote(id peer.ID) *remoteRecord {
	record, exists := sync.store[id]
	if !exists {
		return nil
	}

	return record
}

func (sync *syncModule) loop() {
	ticker := time.NewTicker(time.Minute)
	for {
		<-ticker.C
		go func() {
			log.Infof("checking sync status")

			// do get status
			for _, remotePeerID := range pow.localNode.Peerstore().Peers() {
				go sync.getRemoteStatus(remotePeerID)
			}

			// do sync check
			// convert map to slice first
			slice := make([]*remoteRecord, len(sync.store))
			i := 0
			for _, v := range sync.store {
				slice[i] = v
				i++
			}
			sync.RLock()
			sort.SliceStable(slice, func(i, j int) bool {
				return slice[i].lastChatTime > slice[j].lastChatTime
			})
			for _, r := range slice {
				if r.shouldSync() {
					sync.doSync(r)
				}
			}
			sync.RUnlock()

			// after sync
			sync.PoWork.MiningOn()
		}()
	}
}

func (sync *syncModule) doSync(record *remoteRecord) {
	sync.Lock()
	defer sync.Unlock()

	// get chain
	for sync.chain.GetLatestBlockHeight() < record.latest {
		chain, err := sync.getRemoteChain(record.id)
		if err != nil {
			log.Error(err)
			return
		}

		for i := 0; i < len(chain); i++ {
			err = sync.PoWork.PutNewBlock(chain[i])
			if err != nil {
				log.Error(err)
				return
			}
		}
	}
}

func (sync *syncModule) doInit(record *remoteRecord) {
	sync.Lock()
	defer sync.Unlock()

	// get chain
	for sync.chain.GetLatestBlockHeight() < record.latest {
		chain, err := sync.getRemoteChain(record.id)
		if err != nil {
			log.Error(err)
			return
		}

		for i := 0; i < len(chain); i++ {
			err = sync.PoWork.PutNewBlock(chain[i])
			if err != nil {
				log.Error(err)
				return
			}
		}
	}
}
