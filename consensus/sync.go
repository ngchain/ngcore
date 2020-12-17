package consensus

import (
	"sort"
	"sync"
	"time"

	"github.com/ngchain/ngcore/utils"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngp2p"
)

const (
	minDesiredPeerCount = 3 // TODO: add peer num requirement, avoid mining alone
)

// syncModule is a submodule to the pow, managing the sync of blocks
type syncModule struct {
	pow *PoWork

	localNode *ngp2p.LocalNode

	storeMu sync.RWMutex
	store   map[peer.ID]*RemoteRecord

	*utils.Locker
}

// newSyncModule creates a new sync module
func newSyncModule(pow *PoWork, localNode *ngp2p.LocalNode) *syncModule {
	syncMod := &syncModule{
		pow:       pow,
		localNode: localNode,
		storeMu:   sync.RWMutex{},
		store:     make(map[peer.ID]*RemoteRecord),

		Locker: utils.NewLocker(),
	}

	latest := pow.Chain.GetLatestBlock()
	log.Warnf("current latest block: %x@%d", latest.Hash(), latest.Height)

	return syncMod
}

// put the peer and its remote status into mod
func (mod *syncModule) putRemote(id peer.ID, remote *RemoteRecord) {
	mod.storeMu.Lock()
	defer mod.storeMu.Unlock()
	mod.store[id] = remote
}

// main loop of sync module
func (mod *syncModule) loop() {
	ticker := time.NewTicker(10 * time.Second)

	for {
		<-ticker.C
		log.Infof("checking sync status")

		// do get status
		for _, id := range mod.localNode.Peerstore().Peers() {
			p, _ := mod.localNode.Peerstore().FirstSupportedProtocol(id, string(mod.localNode.GetWiredProtocol()))
			if p == string(mod.localNode.GetWiredProtocol()) && id != mod.localNode.ID() {
				err := mod.getRemoteStatus(id)
				if err != nil {
					log.Warnf("failed to get remote status from %s: %s", id, err)
				}
			}
		}

		// do sync check takes the priority
		// convert map to slice first
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

		var err error
		{
			if records := mod.MustSync(slice); records != nil && len(records) != 0 {
				for _, record := range records {
					err = mod.doSync(record)
					if err != nil {
						log.Warnf("do sync failed: %s, maybe require converging", err)
					} else {
						break
					}
				}
			}
			if err == nil {
				continue
			}

			if records := mod.MustConverge(slice); records != nil && len(records) != 0 {
				for _, record := range records {
					err = mod.doConverging(record)
					if err != nil {
						log.Errorf("converging failed: %s", err)
						record.recordFailure()
					} else {
						break
					}
				}
			}
		}

		// after sync
		mod.pow.SwitchMiningOn()
	}
}
