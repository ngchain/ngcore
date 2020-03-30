package ngp2p

import (
	"github.com/ngchain/ngcore/ngtypes"
	"time"
)

func (w *Wired) UpdateStatus() {
	var total = 0
	var synced = 0
	localHeight := w.node.Chain.GetLatestBlockHeight()

	w.node.RemoteHeights.Range(func(_, value interface{}) bool {
		total++
		if localHeight+ngtypes.BlockCheckRound >= value.(uint64) {
			synced++
		}
		return true
	})

	w.node.isSyncedCh <- float64(synced)/float64(total) > 0.7
}

func (w *Wired) Sync() {
	syncTicker := time.NewTicker(ngtypes.TargetTime)
	defer syncTicker.Stop()

	lastTimeIsSynced := false //default
	for {
		select {
		case <-syncTicker.C:
			for _, peer := range w.node.Peerstore().Peers() {
				if peer == w.node.ID() {
					continue
				}
				log.Infof("pinging to %s", peer)
				w.Ping(peer)
			}
			w.UpdateStatus()
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
