package ngp2p

import (
	"sync"
)

// pattern: /ngp2p/protocol-name/request-or-response-message/version
const (
	protocolVersion = "0.0.2"
	pingMethod      = "/ngp2p/wiredProtocol/ping/" + protocolVersion
	pongMethod      = "/ngp2p/wiredProtocol/pong/" + protocolVersion
	rejectMethod    = "/ngp2p/wiredProtocol/reject/" + protocolVersion
	notFoundMethod  = "/ngp2p/wiredProtocol/notfound/" + protocolVersion

	getChainMethod = "/ngp2p/wiredProtocol/getchain/" + protocolVersion
	chainMethod    = "/ngp2p/wiredProtocol/chain/" + protocolVersion
)

// Wired type
type wiredProtocol struct {
	node     *LocalNode // local host
	requests *sync.Map  // map[string]*pb.Message // used to access request data from response handlers
}

func registerWired(node *LocalNode) *wiredProtocol {
	w := &wiredProtocol{
		node:     node,
		requests: new(sync.Map),
	}

	return w
}
