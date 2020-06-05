package ngp2p

import (
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

// NewHeader is a helper method: generate message data shared between all node's p2p protocols
func (n *LocalNode) NewHeader(uuid []byte, msgType MessageType) *Header {
	peerKey, err := n.Peerstore().PubKey(n.ID()).Bytes()
	if err != nil {
		panic("Failed to get public key for sender from local peer store.")
	}

	return &Header{
		Network:     ngtypes.NETWORK,
		MessageId:   uuid,
		MessageType: msgType,
		Timestamp:   time.Now().Unix(),
		PeerKey:     peerKey,
		Sign:        nil,
	}
}
