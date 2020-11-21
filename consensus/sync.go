package consensus

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngp2p"
)

const (
	minDesiredPeerCount = 3 // TODO: add peer num requirement, avoid mining alone
)

// syncModule is a submodule to the pow, managing the sync of blocks
type syncModule struct {
	sync.RWMutex
	pow *PoWork

	localNode *ngp2p.LocalNode
	store     map[peer.ID]*remoteRecord
}

// newSyncModule creates a new sync module
func newSyncModule(pow *PoWork, localNode *ngp2p.LocalNode) *syncModule {
	syncMod := &syncModule{
		RWMutex: sync.RWMutex{},

		pow:       pow,
		localNode: localNode,
		store:     make(map[peer.ID]*remoteRecord),
	}

	latest := pow.Chain.GetLatestBlock()
	log.Warnf("current latest block: %x@%d", latest.Hash(), latest.Height)

	return syncMod
}

// put the peer and its remote status into mod
func (mod *syncModule) putRemote(id peer.ID, remote *remoteRecord) {
	mod.Lock()
	defer mod.Unlock()
	mod.store[id] = remote
}

// main loop of sync module
func (mod *syncModule) loop() {
	ticker := time.NewTicker(time.Minute)

	for {
		<-ticker.C
		log.Infof("checking sync status")

		// do get status
		for _, id := range mod.localNode.Peerstore().Peers() {
			p, _ := mod.localNode.Peerstore().FirstSupportedProtocol(id, string(mod.localNode.GetWiredProtocol()))
			if p == string(mod.localNode.GetWiredProtocol()) && id != mod.localNode.ID() {
				err := mod.getRemoteStatus(id)
				if err != nil {
					log.Warn(err)
				}
			}
		}

		// do sync check takes the priority
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

		latestHeight := mod.pow.Chain.GetLatestBlockHeight()

		for _, r := range slice {
			if r.shouldSync(latestHeight) {
				err := mod.doSync(r)
				if err != nil {
					log.Warnf("do sync failed: %s, maybe require forking", err)
				}
			}
		}

		// do fork check after sync check
		if shouldFork, r := mod.detectFork(); shouldFork {
			err := mod.doFork(r) // temporarily stuck here
			if err != nil {
				log.Errorf("forking is failed: %s", err)
			}
			continue
		}

		// after sync
		mod.pow.SwitchMiningOn()
	}
}

func (mod *syncModule) doSync(record *remoteRecord) error {
	mod.Lock()
	defer mod.Unlock()

	log.Warnf("start syncing with remote node %s, target height %d", record.id, record.latest)

	// get chain
	for mod.pow.Chain.GetLatestBlockHeight() < record.latest {
		chain, err := mod.getRemoteChainFromLocalLatest(record)
		if err != nil {
			return err
		}

		for i := 0; i < len(chain); i++ {
			err = mod.pow.Chain.ApplyBlock(chain[i])
			if err != nil {
				return fmt.Errorf("failed on applying block@%d: %s", chain[i].Height, err)
			}
		}
	}

	return nil
}
