package consensus

import (
	"fmt"
	"github.com/ngchain/ngcore/ngp2p/defaults"
	"sort"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
)

func (mod *syncModule) bootstrap() {
	peerStore := mod.localNode.Peerstore()

	// init the store
	var wg sync.WaitGroup
	peers := peerStore.Peers()
	localID := mod.localNode.ID()
	for _, id := range peers {
		wg.Add(1)
		go func(id peer.ID) {
			p, _ := mod.localNode.Peerstore().FirstSupportedProtocol(id, defaults.WiredProtocol)
			if p == defaults.WiredProtocol && id != localID {
				err := mod.getRemoteStatus(id)
				if err != nil {
					log.Debug(err)
				}
			}
			wg.Done()
		}(id)
	}
	wg.Wait()

	peerNum := len(mod.store)
	if peerNum < minDesiredPeerCount {
		fmt.Println("lack remote peer for bootstrapping")
	}

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
	latestHeight := mod.pow.Chain.GetLatestBlockHeight()
	for _, r := range slice {
		if r.shouldSync(latestHeight) {
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
	for mod.pow.Chain.GetLatestBlockHeight() < record.latest {
		chain, err := mod.getRemoteChainFromLocalLatest(record.id)
		if err != nil {
			return err
		}

		for i := 0; i < len(chain); i++ {
			err = mod.pow.Chain.ApplyBlock(chain[i])
			if err != nil {
				return err
			}
		}
	}

	return nil
}
