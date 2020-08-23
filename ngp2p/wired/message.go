package wired

import (
	core "github.com/libp2p/go-libp2p-core"
	"github.com/ngchain/ngcore/ngp2p/message"
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

// NewHeader is a helper method: generate message data shared between all node's p2p protocols
func NewHeader(host core.Host, msgID []byte, msgType message.MessageType) *message.Header {
	peerKey, err := host.Peerstore().PubKey(host.ID()).Bytes()
	if err != nil {
		panic("Failed to get public key for sender from local peer store.")
	}

	return &message.Header{
		Network:     ngtypes.NETWORK,
		MessageId:   msgID,
		MessageType: msgType,
		Timestamp:   time.Now().Unix(),
		PeerKey:     peerKey,
		Sign:        nil,
	}
}
