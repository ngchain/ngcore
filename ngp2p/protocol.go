package ngp2p

import (
	"github.com/ngin-network/ngcore/ngtypes"
)

// Protocol type
type Protocol struct {
	node     *LocalNode                     // local host
	requests map[string]*ngtypes.P2PMessage // used to access request data from response handlers
	doneCh   chan bool                      // only for demo purposes to stop main from terminating
}

func RegisterProtocol(node *LocalNode, done chan bool) *Protocol {
	p := &Protocol{
		node:     node,
		requests: make(map[string]*ngtypes.P2PMessage),
		doneCh:   done,
	}
	// register handlers
	node.SetStreamHandler(pingMethod, p.onPing)
	node.SetStreamHandler(pongMethod, p.onPong)
	node.SetStreamHandler(rejectMethod, p.onReject)

	node.SetStreamHandler(getChainMethod, p.onGetChain)
	node.SetStreamHandler(chainMethod, p.onChain)

	return p
}
