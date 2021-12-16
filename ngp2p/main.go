package ngp2p

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	multiplex "github.com/libp2p/go-libp2p-mplex"
	yamux "github.com/libp2p/go-libp2p-yamux"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"github.com/libp2p/go-tcp-transport"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngp2p/broadcast"
	"github.com/ngchain/ngcore/ngp2p/wired"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("ngp2p")

// LocalNode is the local host on p2p network
type LocalNode struct {
	host.Host // lib-p2p host
	network   ngtypes.Network
	P2PConfig P2PConfig

	*wired.Wired
	*broadcast.Broadcast
}

type P2PConfig struct {
	P2PKeyFile       string
	Network          ngtypes.Network
	Port             int
	DisableDiscovery bool
}

// InitLocalNode creates a new node with its implemented protocols.
func InitLocalNode(chain *blockchain.Chain, config P2PConfig) *LocalNode {
	ctx := context.Background()
	priv := keytools.GetP2PKey(config.P2PKeyFile)

	transports := libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
		// libp2p.Transport(ws.New),
	)

	listenAddrs := libp2p.ListenAddrStrings(
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", config.Port),
		fmt.Sprintf("/ip6/::/tcp/%d", config.Port),
	)

	muxers := libp2p.ChainOptions(
		libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
		libp2p.Muxer("/mplex/6.7.0", multiplex.DefaultTransport),
	)

	localHost, err := libp2p.New(
		transports,
		listenAddrs,
		muxers,
		libp2p.Identity(priv),
		getPublicRouter(config.Network),
		libp2p.NATPortMap(),
		libp2p.EnableAutoRelay(),
	)
	if err != nil {
		panic(err)
	}

	// init
	log.Warnf("P2P Listening on: /ip4/<External IP>/tcp/%d/p2p/%s \n", config.Port, localHost.ID().String())

	localNode := &LocalNode{
		// sub modules
		Host:      rhost.Wrap(localHost, p2pDHT),
		network:   config.Network,
		Wired:     wired.NewWiredProtocol(localHost, config.Network, chain),
		Broadcast: broadcast.NewBroadcastProtocol(localHost, config.Network, make(chan *ngtypes.FullBlock), make(chan *ngtypes.FullTx)),
	}

	if !config.DisableDiscovery {
		initMDNS(ctx, localHost)
		activeDHT(ctx, p2pDHT, localNode)
	}

	return localNode
}

func (localNode *LocalNode) GoServe() {
	localNode.Wired.GoServe()
	localNode.Broadcast.GoServe()
}
