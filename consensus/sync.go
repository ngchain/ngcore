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
	pow *PoWork

	store map[peer.ID]*remoteRecord
}

func newSyncModule(pow *PoWork, isBootstrapNode bool) *syncModule {
	syncMod := &syncModule{
		RWMutex: sync.RWMutex{},
		pow:     pow,
		store:   make(map[peer.ID]*remoteRecord),
	}

	if !isBootstrapNode {
		syncMod.bootstrap()
	}

	return syncMod
}

func (mod *syncModule) putRemote(id peer.ID, remote *remoteRecord) {
	mod.Lock()
	defer mod.Unlock()
	mod.store[id] = remote
}

func (mod *syncModule) getRemote(id peer.ID) *remoteRecord {
	record, exists := mod.store[id]
	if !exists {
		return nil
	}

	return record
}

func (mod *syncModule) loop() {
	ticker := time.NewTicker(time.Minute)

	for {
		<-ticker.C
		log.Infof("checking sync status")

		// do get status
		for _, id := range ngp2p.GetLocalNode().Peerstore().Peers() {
			p, _ := ngp2p.GetLocalNode().Peerstore().FirstSupportedProtocol(id, ngp2p.WiredProtocol)
			if p == ngp2p.WiredProtocol && id != ngp2p.GetLocalNode().ID() {
				err := mod.getRemoteStatus(id)
				if err != nil {
					log.Warn(err)
				}
			}
		}

		// do fork check
		if shouldFork, r := mod.detectFork(); shouldFork {
			err := mod.doFork(r) // temporarily stuck here
			if err != nil {
				log.Warnf("forking is failed: %s", err)
			}
		}

		// do sync check
		// convert map to slice first
		slice := make([]*remoteRecord, len(mod.store))
		i := 0

		for _, v := range mod.store {
			slice[i] = v
			i++
		}

		sort.SliceStable(slice, func(i, j int) bool {
			return slice[i].lastChatTime > slice[j].lastChatTime
		})

		for _, r := range slice {
			if r.shouldSync() {
				err := mod.doSync(r)
				if err != nil {
					log.Warnf("do sync failed: %s", err)
				}
			}
		}

		// after sync
		mod.pow.MiningOn()
	}
}

func (mod *syncModule) doSync(record *remoteRecord) error {
	mod.Lock()
	defer mod.Unlock()

	log.Warnf("start syncing with remote node %s", record.id)

	// get chain
	for storage.GetChain().GetLatestBlockHeight() < record.latest {
		chain, err := mod.getRemoteChainFromLocalLatest(record.id)
		if err != nil {
			return err
		}

		for i := 0; i < len(chain); i++ {
			err = GetPoWConsensus().ApplyBlock(chain[i])
			if err != nil {
				return err
			}
		}
	}

	return nil
}
