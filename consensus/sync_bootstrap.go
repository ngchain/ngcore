package consensus

import (
	"sort"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/storage"
)

func (mod *syncModule) bootstrap() {
	peerStore := ngp2p.GetLocalNode().Peerstore()

	// init the store
	var wg sync.WaitGroup
	for _, id := range peerStore.Peers() {
		wg.Add(1)
		go func(id peer.ID) {
			p, _ := ngp2p.GetLocalNode().Peerstore().FirstSupportedProtocol(id, ngp2p.WiredProtocol)
			if p == ngp2p.WiredProtocol && id != ngp2p.GetLocalNode().ID() {
				err := mod.getRemoteStatus(id)
				if err != nil {
					log.Debug(err)
				}
			}
			wg.Done()
		}(id)
	}
	wg.Wait()

	slice := make([]*remoteRecord, len(mod.store))
	i := 0

	for _, v := range mod.store {
		slice[i] = v
		i++
	}

	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].lastChatTime > slice[j].lastChatTime
	})

	// initial sync
	for _, r := range slice {
		if r.shouldSync() {
			err := mod.doInit(r)
			if err != nil {
				panic(err)
			}
		}
	}

	latest := storage.GetChain().GetLatestBlock()
	log.Warnf("completed sync, latest height@%d: %x", latest.Height, latest.Hash())

	// then check fork
	if shouldFork, r := mod.detectFork(); shouldFork {
		err := mod.doFork(r) // temporarily stuck here
		if err != nil {
			log.Errorf("forking is failed: %s", err)
		}
	}
}

func (mod *syncModule) doInit(record *remoteRecord) error {
	mod.Lock()
	defer mod.Unlock()

	log.Warnf("start initial syncing with remote node %s", record.id)

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
