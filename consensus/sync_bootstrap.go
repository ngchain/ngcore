package consensus

import (
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
			p, _ := mod.localNode.Peerstore().FirstSupportedProtocol(id, string(mod.localNode.GetWiredProtocol()))
			if p == string(mod.localNode.GetWiredProtocol()) && id != localID {
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
		log.Warnf("lack remote peer for bootstrapping")
		// TODO: when peer count is less than the minDesiredPeerCount, the consensus shouldn't do any sync nor fork
	}

	slice := make([]*RemoteRecord, len(mod.store))
	i := 0

	for _, v := range mod.store {
		slice[i] = v
		i++
	}

	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].lastChatTime > slice[j].lastChatTime
	})
	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].latest > slice[j].latest
	})

	// catch error
	var err error
	{
		if records := mod.MustSync(slice); records != nil && len(records) != 0 {
			for _, record := range records {
				err = mod.doSync(record)
				if err != nil {
					log.Warnf("do sync failed: %s, maybe require forking", err)
				} else {
					break
				}
			}
		}

		// do fork check after sync check
		if records := mod.MustFork(slice); records != nil && len(records) != 0 {
			for _, record := range records {
				err = mod.doFork(record)
				if err != nil {
					log.Errorf("forking is failed: %s", err)
					record.recordFailure()
				} else {
					break
				}
			}
		}
	}
	if err != nil {
		panic(err)
	}
}

func (mod *syncModule) doInit(record *RemoteRecord) error {
	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("initial syncing with remote node %s", record.id)

	// get chain
	for mod.pow.Chain.GetLatestBlockHeight() < record.latest {
		chain, err := mod.getRemoteChainFromLocalLatest(record)
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
