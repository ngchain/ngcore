package ngp2p

import (
	"sync"
)

// Wired type
type wired struct {
	node     *LocalNode // local host
	requests *sync.Map  //map[string]*pb.Message // used to access request data from response handlers
}

func registerProtocol(node *LocalNode) *wired {
	p := &wired{
		node:     node,
		requests: new(sync.Map),
	}
	// register handlers
	node.SetStreamHandler(pingMethod, p.onPing)
	node.SetStreamHandler(pongMethod, p.onPong)
	node.SetStreamHandler(rejectMethod, p.onReject)
	node.SetStreamHandler(notfoundMethod, p.onNotFound)

	node.SetStreamHandler(getChainMethod, p.onGetChain)
	node.SetStreamHandler(chainMethod, p.onChain)

	return p
}
