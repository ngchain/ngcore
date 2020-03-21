package ngp2p

import (
	"github.com/ngchain/ngcore/ngtypes"
	"time"
)

func (w *Wired) Sync() {
	syncInterval := time.Tick(ngtypes.TargetTime * ngtypes.BlockCheckRound)
	for {
		select {
		case <-syncInterval:
			for _, peer := range w.node.Peerstore().Peers() {
				if peer == w.node.ID() {
					continue
				}
				log.Infof("pinging to %s", peer)
				w.Ping(peer)
			}
		}
	}
}
