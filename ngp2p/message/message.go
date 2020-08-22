package message

import (
	logging "github.com/ipfs/go-log/v2"
	core "github.com/libp2p/go-libp2p-core"
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("msg")

// NewHeader is a helper method: generate message data shared between all node's p2p protocols
func NewHeader(host core.Host, msgID []byte, msgType MessageType) *Header {
	peerKey, err := host.Peerstore().PubKey(host.ID()).Bytes()
	if err != nil {
		panic("Failed to get public key for sender from local peer store.")
	}

	return &Header{
		Network:     ngtypes.NETWORK,
		MessageId:   msgID,
		MessageType: msgType,
		Timestamp:   time.Now().Unix(),
		PeerKey:     peerKey,
		Sign:        nil,
	}
}
