package ngp2p

import (
	"github.com/ngchain/ngcore/ngp2p/pb"
)

// Wired type
type Wired struct {
	node     *LocalNode             // local host
	requests map[string]*pb.Message // used to access request data from response handlers
}

func registerProtocol(node *LocalNode) *Wired {
	p := &Wired{
		node:     node,
		requests: make(map[string]*pb.Message),
	}
	// register handlers
	node.SetStreamHandler(pingMethod, p.onPing)
	node.SetStreamHandler(pongMethod, p.onPong)
	node.SetStreamHandler(rejectMethod, p.onReject)

	node.SetStreamHandler(getChainMethod, p.onGetChain)
	node.SetStreamHandler(chainMethod, p.onChain)

	return p
}
