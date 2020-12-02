package consensus

import (
	"fmt"
	"github.com/ngchain/ngcore/ngtypes"
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
		sort.SliceStable(slice, func(i, j int) bool {
			return slice[i].latest > slice[j].latest
		})

		var err error
		{
			if r := mod.MustSync(slice); r != nil {
				err = mod.doSync(r)
				if err != nil {
					log.Warnf("do sync failed: %s, maybe require forking", err)
				}
			}

			// do fork check after sync check
			if r := mod.MustFork(slice); r != nil {
				err = mod.doFork(r)
				if err != nil {
					log.Errorf("forking is failed: %s", err)
					r.recordFailure()
				}
				continue
			}
		}

		// after sync
		mod.pow.SwitchMiningOn()
	}
}

// RULE: checkpoint fork: when a node mined a checkpoint, all other node are forced to start sync
func (mod *syncModule) MustSync(recordSlice []*remoteRecord) *remoteRecord {
	latestHeight := mod.pow.Chain.GetLatestBlockHeight()

	if recordSlice[0].latest/ngtypes.BlockCheckRound > latestHeight/ngtypes.BlockCheckRound {
		return recordSlice[0]
	}

	return nil
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

	height := mod.pow.Chain.GetLatestBlockHeight()
	log.Warnf("sync finished with remote node %s, local height %d", record.id, height)

	return nil
}
