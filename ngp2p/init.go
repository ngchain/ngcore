package ngp2p

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

func (n *LocalNode) connectBootstrapNodes() {
	ctx := context.Background()
	for i := range bootstrapNodes {
		targetAddr, err := multiaddr.NewMultiaddr(bootstrapNodes[i])
		if err != nil {
			panic(err)
		}

		targetInfo, err := peer.AddrInfoFromP2pAddr(targetAddr)
		if err != nil {
			panic(err)
		}

		err = n.Connect(ctx, *targetInfo)
		if err != nil {
			panic(err)
		}

		n.ping(targetInfo.ID)
	}
}

// Init will initialize the LocalNode by connecting to the bootstrap nodes
func (n *LocalNode) Init() {
	if !n.isBootstrapNode {
		n.connectBootstrapNodes()
	}
}
