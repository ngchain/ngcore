package ngp2p

import (
	"context"
	"fmt"

	multiplex "github.com/libp2p/go-libp2p-mplex"
	yamux "github.com/libp2p/go-libp2p-yamux"
	"github.com/ngchain/ngcore/ngp2p/broadcast"
	"github.com/ngchain/ngcore/ngp2p/wired"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"github.com/libp2p/go-tcp-transport"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("ngp2p")

// LocalNode is the local host on p2p network
type LocalNode struct {
	host.Host // lib-p2p host
	*wired.Wired
	*broadcast.Broadcast
}

var localNode *LocalNode

// InitLocalNode creates a new node with its implemented protocols.
func InitLocalNode(port int) {
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
	fmt.Printf("P2P Listening on: /ip4/<External IP>/tcp/%d/p2p/%s \n", port, localHost.ID().String())

	initMDNS(ctx, localHost)

	localNode = &LocalNode{
		// sub modules
		Host:      rhost.Wrap(localHost, p2pDHT),
		Wired:     wired.NewWiredProtocol(localHost),
		Broadcast: broadcast.NewBroadcastProtocol(localHost, make(chan *ngtypes.Block), make(chan *ngtypes.Tx)),
	}

	activeDHT(ctx, p2pDHT, localNode)
}

func GoServe() {
	localNode.Wired.GoServe()
	localNode.Broadcast.GoServe()
}

// GetLocalNode returns the initialized LocalNode in module.
func GetLocalNode() *LocalNode {
	if localNode == nil {
		panic("localNode is closed")
	}

	return localNode
}
