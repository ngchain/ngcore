package consensus

import (
	"fmt"
	"sort"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

func (mod *syncModule) bootstrap() {
	log.Warn("bootstrapping ... ")
	peerStore := mod.localNode.Peerstore()

	// init the store
	var wg sync.WaitGroup
	peers := peerStore.Peers()
	localID := mod.localNode.ID()
	for _, id := range peers {
		wg.Add(1)
		go func(id peer.ID) {
			p, _ := mod.localNode.Peerstore().FirstSupportedProtocol(id, mod.localNode.GetWiredProtocol())
			if p == mod.localNode.GetWiredProtocol() && id != localID {
				err := mod.getRemoteStatus(id)
				if err != nil {
					log.Debug(err)
				}
			}
			wg.Done()
		}(id)
	}
	wg.Wait()

	slice := mod.remotesSnapshot()
	if len(slice) < minDesiredPeerCount {
		log.Warnf("lack remote peer for bootstrapping, current peer num: %d", len(slice))
		// TODO: when peer count is less than the minDesiredPeerCount, the consensus shouldn't do any sync nor converge
	}

	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].lastChatTime > slice[j].lastChatTime
	})
	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].latest > slice[j].latest
	})

	// catch error
	var err error
	if records := mod.MustSync(slice); len(records) != 0 {
		for _, record := range records {
			if !mod.pow.StrictMode && mod.pow.Chain.GetLatestBlockHeight() == 0 {
				//
				err = mod.switchToRemoteCheckpoint(record)
				if err != nil {
					panic(fmt.Errorf("failed to fast sync via checkpoint: %w", err))
				}
			}

			if mod.pow.SnapshotMode {
				err = mod.doSnapshotSync(record)
			} else {
				err = mod.doSync(record)
			}

			if err != nil {
				log.Warnf("do sync failed: %s, maybe require converging", err)
			} else {
				break
			}
		}
	}

	// do converge check after sync check
	if records := mod.MustConverge(slice); len(records) != 0 {
		for _, record := range records {
			if mod.pow.SnapshotMode {
				err = mod.doSnapshotConverging(record)
			} else {
				err = mod.doConverging(record)
			}

			if err != nil {
				log.Errorf("converging failed: %s", err)
				record.recordFailure()
			} else {
				break
			}
		}
	}
	if err != nil {
		panic(err)
	}
}
