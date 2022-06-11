package ngp2p

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
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
	peerInfoCh := make(chan peer.AddrInfo)
	service := mdns.NewMdnsService(localHost, "", &mdnsNotifee{
		h:          localHost,
		PeerInfoCh: peerInfoCh,
	}) // using ipfs network
	service.Start()

	go func() {
		for {
			pi := <-peerInfoCh // will block until we discover a peer
			log.Debugf("Found peer: %s, connecting", pi.String())

			if pi.ID == localHost.ID() {
				continue
			}

			if err := localHost.Connect(ctx, pi); err != nil {
				log.Errorf("Connection failed: %s", err)
				continue
			}
		}
	}()

	return peerInfoCh
}
