package ngp2p

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// DefaultMinPeers is the connection count the peer manager tries to keep
const DefaultMinPeers = 3

// peerManagerLoop keeps the node connected: whenever the live connection
// count drops below MinPeers, it redials the bootstrap nodes and every
// known address from the peerstore. It exits when the node closes
func (localNode *LocalNode) peerManagerLoop(ctx context.Context) {
	interval := localNode.P2PConfig.ReconnectInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}

		minPeers := localNode.P2PConfig.MinPeers
		if minPeers <= 0 {
			minPeers = DefaultMinPeers
		}

		connected := localNode.Network().Peers()
		if len(connected) >= minPeers {
			continue
		}

		log.Debugf("connected to %d/%d peers, redialing known peers", len(connected), minPeers)
		localNode.redialKnownPeers(ctx)
	}
}

// redialKnownPeers dials the bootstrap nodes plus every peerstore entry
// with known addresses which is not connected right now. It WAITS for the
// dials to finish so the shared dial context outlives them — spawning the
// dials and returning immediately would let the deferred cancel abort them
func (localNode *LocalNode) redialKnownPeers(ctx context.Context) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	dial := func(pi peer.AddrInfo) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := localNode.Connect(dialCtx, pi); err != nil {
				log.Debugf("failed to redial peer %s: %v", pi.ID, err)
			}
		}()
	}

	// bootstrap nodes first
	if !localNode.P2PConfig.DisableConnectingBootstraps {
		for _, ma := range BootstrapNodes {
			pi, err := peer.AddrInfoFromP2pAddr(ma)
			if err != nil {
				continue
			}
			if localNode.Network().Connectedness(pi.ID) == network.Connected {
				continue
			}
			dial(*pi)
		}
	}

	// then every known peer from past sessions/connections
	for _, id := range localNode.Peerstore().PeersWithAddrs() {
		if id == localNode.ID() || localNode.Network().Connectedness(id) == network.Connected {
			continue
		}

		addrs := localNode.Peerstore().Addrs(id)
		if len(addrs) == 0 {
			continue
		}
		dial(peer.AddrInfo{ID: id, Addrs: addrs})
	}

	wg.Wait()
}
