package ngp2p

import (
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

func (w *wired) updateStatus() {
	total := 0
	synced := 0
	localHeight := w.node.consensus.GetLatestBlockHeight()

	w.node.RemoteHeights.Range(func(_, value interface{}) bool {
		total++
		if localHeight+ngtypes.BlockCheckRound >= value.(uint64) {
			synced++
		}
		return true
	})

	progress := float64(synced) / float64(total)
	w.node.isSyncedCh <- progress > 0.9
}

func (w *wired) sync() {
	syncTicker := time.NewTicker(ngtypes.TargetTime)
	defer syncTicker.Stop()

	lastTimeIsSynced := false // default

	for {
		select {
		case <-syncTicker.C:
			for _, peer := range w.node.Peerstore().Peers() {
				if peer == w.node.ID() {
					continue
				}
				log.Debugf("pinging to %s", peer)
				w.ping(peer)
			}

			go w.updateStatus()
		case isSynced := <-w.node.isSyncedCh:
			if isSynced && !lastTimeIsSynced {
				log.Info("localnode is synced with network")
				if w.node.OnSynced != nil {
					w.node.OnSynced()
				}
			}

			if !isSynced && lastTimeIsSynced {
				log.Info("localnode is not synced with network, syncing...")
				if w.node.OnNotSynced != nil {
					w.node.OnNotSynced()
				}
			}

			lastTimeIsSynced = isSynced
		}
	}
}
