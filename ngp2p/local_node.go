package ngp2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	multiplex "github.com/libp2p/go-libp2p-mplex"
	yamux "github.com/libp2p/go-libp2p-yamux"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	"github.com/libp2p/go-tcp-transport"
	"github.com/ngchain/ngcore/ngtypes"
)

// LocalNode is the local host on p2p network
type LocalNode struct {
	host.Host // lib-p2p host
	*wiredProtocol
	*broadcastProtocol

	OnBlock chan *ngtypes.Block
	OnTx    chan *ngtypes.Tx
}

var localNode *LocalNode

// NewLocalNode creates a new node with its implemented protocols.
func NewLocalNode(port int) *LocalNode {
	if localNode == nil {
		ctx := context.Background()
		priv := getP2PKey()

		transports := libp2p.ChainOptions(
			libp2p.Transport(tcp.NewTCPTransport),
			// libp2p.Transport(ws.New),
		)

		listenAddrs := libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
			fmt.Sprintf("/ip6/::/tcp/%d", port),
		)

		muxers := libp2p.ChainOptions(
			libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
			libp2p.Muxer("/mplex/6.7.0", multiplex.DefaultTransport),
		)

		var p2pDHT *dht.IpfsDHT

		newDHT := func(h host.Host) (routing.PeerRouting, error) {
			var err error
			p2pDHT, err = dht.New(ctx, h)

			return p2pDHT, err
		}

		localHost, err := libp2p.New(
			ctx,
			transports,
			listenAddrs,
			muxers,
			libp2p.Identity(priv),
			libp2p.Routing(newDHT),
			libp2p.NATPortMap(),
			libp2p.EnableAutoRelay(),
		)
		if err != nil {
			panic(err)
		}

		// init
		for _, addr := range localHost.Addrs() {
			log.Infof("Listening P2P on %s/p2p/%s", addr.String(), localHost.ID().String())
		}

		mdns, err := discovery.NewMdnsService(ctx, localHost, time.Second*10, "") // using ipfs network
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

		localNode = &LocalNode{
			// sub modules
			Host:              localHost,
			wiredProtocol:     nil,
			broadcastProtocol: nil,

			// events
			OnBlock: make(chan *ngtypes.Block, 1),
			OnTx:    make(chan *ngtypes.Tx, 1),
		}

		localNode.broadcastProtocol = registerBroadcaster(localNode)
		localNode.wiredProtocol = registerWired(localNode)

		// mdns seeding
		go func() {
			for {
				pi := <-peerInfoCh // will block until we discover a peer
				log.Infof("Found peer:", pi, ", connecting")

				if err = localNode.Connect(ctx, pi); err != nil {
					log.Errorf("Connection failed: %s", err)
					continue
				}
			}
		}()

		err = p2pDHT.Bootstrap(ctx)
		if err != nil {
			panic(err)
		}
	}

	return localNode
}

// GetLocalNode returns the initialized LocalNode in module.
func GetLocalNode() *LocalNode {
	if localNode == nil {
		panic("localNode is closed")
	}

	return localNode
}

// PrivKey is a helper func for getting private key from local peer id
func (n *LocalNode) PrivKey() crypto.PrivKey {
	return n.Peerstore().PrivKey(n.ID())
}
