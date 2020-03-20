package ngp2p

import (
	"github.com/ngin-network/ngcore/ngtypes"
	"time"
)

func (w *Wired) Sync() {
	syncInterval := time.Tick(ngtypes.TargetTime * ngtypes.BlockCheckRound)
	for {
		select {
		case <-syncInterval:
			for _, peer := range w.node.Peerstore().Peers() {
				log.Infof("pinging to %s", peer)
				w.Ping(peer)
			}
		}
	}
}
