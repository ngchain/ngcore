package ngp2p

import (
	"sync"

	"go.uber.org/atomic"
)

// Wired type
type wired struct {
	node        *LocalNode // local host
	requests    *sync.Map  // map[string]*pb.Message // used to access request data from response handlers
	forkManager *forkManager
}

func registerWired(node *LocalNode) *wired {
	w := &wired{
		node:     node,
		requests: new(sync.Map),
	}

	w.forkManager = &forkManager{
		w:       w,
		enabled: atomic.NewBool(true),
	}

	// register handlers
	node.SetStreamHandler(pingMethod, w.onPing)
	node.SetStreamHandler(pongMethod, w.onPong)
	node.SetStreamHandler(rejectMethod, w.onReject)
	node.SetStreamHandler(notfoundMethod, w.onNotFound)

	node.SetStreamHandler(getChainMethod, w.onGetChain)
	node.SetStreamHandler(chainMethod, w.onChain)

	return w
}
