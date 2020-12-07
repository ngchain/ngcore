package ngp2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery"
)

type mdnsNotifee struct {
	h          host.Host
	PeerInfoCh chan peer.AddrInfo
}

// HandlePeerFound is required for mdnsNotifee to be a Notifee.
func (m *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	m.PeerInfoCh <- pi
}

// for private net connection
func initMDNS(ctx context.Context, localHost host.Host) chan peer.AddrInfo {
	mdns, err := discovery.NewMdnsService(ctx, localHost, 10*time.Second, "") // using ipfs network
	if err != nil {
		panic(err)
	}

	peerInfoCh := make(chan peer.AddrInfo)

	mdns.RegisterNotifee(
		&mdnsNotifee{
			h:          localHost,
			PeerInfoCh: peerInfoCh,
		},
	)

	go func() {
		for {
			pi := <-peerInfoCh // will block until we discover a peer
			log.Debugf("Found peer: %s, connecting", pi.String())

			if err = localHost.Connect(ctx, pi); err != nil {
				log.Errorf("Connection failed: %s", err)
				continue
			}
		}
	}()

	return peerInfoCh
}
