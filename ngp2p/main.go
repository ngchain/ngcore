package ngp2p

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	multiplex "github.com/libp2p/go-libp2p-mplex"
	yamux "github.com/libp2p/go-libp2p-yamux"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"github.com/libp2p/go-tcp-transport"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("ngp2p")

// LocalNode is the local host on p2p network
type LocalNode struct {
	host.Host // lib-p2p host
	*wiredProtocol
	*broadcastProtocol

	OnBlock chan *ngtypes.Block // TODO: add queue for receiving
	OnTx    chan *ngtypes.Tx
}

var localNode *LocalNode

// NewLocalNode creates a new node with its implemented protocols.
func NewLocalNode(port int) *LocalNode {
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

	localHost, err := libp2p.New(
		ctx,
		transports,
		listenAddrs,
		muxers,
		libp2p.Identity(priv),
		getPublicRouter(),
		libp2p.NATPortMap(),
		libp2p.EnableAutoRelay(),
	)
	if err != nil {
		panic(err)
	}

	// init
	for _, addr := range localHost.Addrs() {
		fmt.Printf("P2P Listening on: \t%s/p2p/%s \n", addr.String(), localHost.ID().String())
	}

	initMDNS(ctx, localHost)

	localNode = &LocalNode{
		// sub modules
		Host:              rhost.Wrap(localHost, p2pDHT),
		wiredProtocol:     nil,
		broadcastProtocol: nil,

		// events
		OnBlock: make(chan *ngtypes.Block, 0),
		OnTx:    make(chan *ngtypes.Tx, 0),
	}

	localNode.broadcastProtocol = registerBroadcaster(localNode)
	localNode.wiredProtocol = registerWired(localNode)

	activeDHT(ctx, p2pDHT, localNode)

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
