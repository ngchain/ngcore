package ngp2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	yamux "github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/keytools"
	"github.com/ngchain/ngcore/ngp2p/broadcast"
	"github.com/ngchain/ngcore/ngp2p/wired"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("ngp2p")

var _ host.Host = (*LocalNode)(nil)

// LocalNode is the local host on p2p network
type LocalNode struct {
	host.Host // lib-p2p host
	network   ngtypes.Network
	P2PConfig P2PConfig

	ctx    context.Context
	cancel context.CancelFunc

	*wired.Wired
	*broadcast.Broadcast
}

type P2PConfig struct {
	P2PKeyFile                  string
	Network                     ngtypes.Network
	Port                        int
	DisableDiscovery            bool
	DisableConnectingBootstraps bool

	// MinPeers is the connection count the peer manager keeps alive by
	// redialing known peers (0 = DefaultMinPeers)
	MinPeers int
	// ReconnectInterval is the peer manager's check cadence (0 = 15s)
	ReconnectInterval time.Duration
}

// InitLocalNode creates a new node with its implemented protocols.
func InitLocalNode(chain *blockchain.Chain, config P2PConfig) *LocalNode {
	ctx, cancel := context.WithCancel(context.Background())
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
	)

	localHost, err := libp2p.New(
		transports,
		listenAddrs,
		muxers,
		libp2p.Identity(priv),
		getPublicRouter(config.Network),
		libp2p.NATPortMap(),
		// libp2p.EnableAutoRelay(),
	)
	if err != nil {
		panic(err)
	}

	// init
	log.Warnf("P2P Listening on: /ip4/<External IP>/tcp/%d/p2p/%s \n", config.Port, localHost.ID().String())

	localNode := &LocalNode{
		// sub modules
		Host: rhost.Wrap(localHost, p2pDHT),
		// the peer manager reads MinPeers / ReconnectInterval /
		// DisableConnectingBootstraps off this: without it every setting is
		// the zero value, so a user who DISABLED bootstrap dialing still
		// gets the public bootstrap nodes redialed
		P2PConfig: config,
		network:   config.Network,
		ctx:       ctx,
		cancel:    cancel,
		Wired:     wired.NewWiredProtocol(localHost, config.Network, chain),
		Broadcast: broadcast.NewBroadcastProtocol(localHost, config.Network, make(chan *ngtypes.FullBlock), make(chan *ngtypes.FullTx), make(chan *ngtypes.Commitment)),
	}

	if !config.DisableDiscovery {
		activeDHT(ctx, p2pDHT, localNode, config.DisableConnectingBootstraps)
	}

	return localNode
}

func (localNode *LocalNode) GoServe() {
	localNode.Wired.GoServe()
	localNode.Broadcast.GoServe()

	go localNode.peerManagerLoop(localNode.ctx)
}

// Close shuts the p2p node down: stops the broadcast listeners, leaves
// the pubsub topics and closes the libp2p host with all its streams.
// It also resolves the Close ambiguity between the embedded Host and
// Broadcast, keeping *LocalNode a valid host.Host
func (localNode *LocalNode) Close() error {
	localNode.cancel()
	localNode.Broadcast.Close()

	return localNode.Host.Close()
}
