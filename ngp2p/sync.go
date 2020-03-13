package ngp2p

import (
	"github.com/ngin-network/ngcore/ngtypes"
	"log"
	"time"
)

func (p *Protocol) Sync() {
	syncInterval := time.Tick(ngtypes.TargetTime * ngtypes.CheckRound)
	for {
		select {
		case <-syncInterval:
			for _, peer := range p.node.Peerstore().Peers() {
				log.Printf("pinging to %s", peer)
				p.Ping(peer)
			}
		}
	}
}
