package consensus

import (
	"fmt"
	"sort"
	"sync"

	"github.com/ngchain/ngcore/ngchain"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/ngchain/ngcore/ngp2p"
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

	fmt.Printf("Start initial syncing with remote node: %s\n", record.id)
	log.Warnf("Start initial syncing with remote node %s", record.id)

	// get chain
	for ngchain.GetLatestBlockHeight() < record.latest {
		chain, err := mod.getRemoteChainFromLocalLatest(record.id)
		if err != nil {
			return err
		}

		for i := 0; i < len(chain); i++ {
			err = ngchain.ApplyBlock(chain[i])
			if err != nil {
				return err
			}
		}
	}

	return nil
}
