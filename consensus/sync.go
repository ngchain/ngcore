package consensus

import (
	"sort"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngp2p"
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
	if r.latest > storage.GetChain().GetLatestBlockHeight() {
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
	defer sync.Unlock()
	sync.store[id] = remote
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
			for _, id := range ngp2p.GetLocalNode().Peerstore().Peers() {
				if id != ngp2p.GetLocalNode().ID() {
					err := sync.getRemoteStatus(id)
					if err != nil {
						log.Warn(err)
					}
				}
			}

			// do sync check
			// convert map to slice first
			slice := make([]*remoteRecord, len(sync.store))
			i := 0

			for _, v := range sync.store {
				slice[i] = v
				i++
			}

			sort.SliceStable(slice, func(i, j int) bool {
				return slice[i].lastChatTime > slice[j].lastChatTime
			})

			for _, r := range slice {
				if r.shouldSync() {
					sync.doSync(r)
				}
			}

			// after sync
			sync.PoWork.MiningOn()
		}()
	}
}

func (sync *syncModule) doSync(record *remoteRecord) error {
	sync.Lock()
	defer sync.Unlock()

	log.Warnf("start syncing with remote node %s", record.id)

	// get chain
	for storage.GetChain().GetLatestBlockHeight() < record.latest {
		chain, err := sync.getRemoteChain(record.id)
		if err != nil {
			return err
		}

		for i := 0; i < len(chain); i++ {
			err = storage.GetChain().PutNewBlock(chain[i])
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (sync *syncModule) doInit(record *remoteRecord) error {
	sync.Lock()
	defer sync.Unlock()

	log.Warnf("start initial syncing with remote node %s", record.id)

	// get chain
	for storage.GetChain().GetLatestBlockHeight() < record.latest {
		chain, err := sync.getRemoteChain(record.id)
		if err != nil {
			return err
		}

		for i := 0; i < len(chain); i++ {
			err = storage.GetChain().PutNewBlock(chain[i])
			if err != nil {
				return err
			}
		}
	}

	return nil
}
