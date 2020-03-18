package ngp2p

import (
	"github.com/ngin-network/ngcore/ngtypes"
	"time"
)

func (p *Protocol) Sync() {
	syncInterval := time.Tick(ngtypes.TargetTime * ngtypes.BlockCheckRound)
	for {
		select {
		case <-syncInterval:
			for _, peer := range p.node.Peerstore().Peers() {
				log.Infof("pinging to %s", peer)
				p.Ping(peer)
			}
		}
	}
}
