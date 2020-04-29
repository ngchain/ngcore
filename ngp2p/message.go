package ngp2p

import (
	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
)

// NewHeader is a helper method: generate message data shared between all node's p2p protocols
// messageId: unique for requests, copied from request for responses.
func (n *LocalNode) NewHeader(uuid string) *pb.Header {
	// Add protobufs bin data for message author public key
	// this is useful for authenticating  messages forwarded by a node authored by another node
	peerKey, err := n.Peerstore().PubKey(n.ID()).Bytes()

	if err != nil {
		panic("Failed to get public key for sender from local peer store.")
	}

	return &pb.Header{
		NetworkId: ngtypes.Network,
		Uuid:      uuid,
		Timestamp: 0,
		PeerKey:   peerKey,
		Sign:      nil,
	}
}
