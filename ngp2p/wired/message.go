package wired

import (
	"fmt"
	"github.com/ngchain/ngcore/ngtypes"
	"time"

	core "github.com/libp2p/go-libp2p-core"
)

type MsgType uint8

const (
	InvalidMsg MsgType = iota
	PingMsg
	PongMsg
	RejectMsg
	//MsgNotFound - deleted because Reject can cover not-found msg
)

func (mt MsgType) String() string {
	switch mt {
	case InvalidMsg:
		return "InvalidMsg"
	case PingMsg:
		return "PingMsg"
	case PongMsg:
		return "PongMsg"
	case RejectMsg:
		return "RejectMsg"
	default:
		return fmt.Sprintf("UnknownMsg: %d", mt)
	}
}

const (
	GetChainMsg MsgType = iota + 0x10
	ChainMsg
	GetSheetMsg
	SheetMsg
)

type ChainType uint8

const (
	InvalidChain ChainType = iota
	BlockChain
	HeaderChain
	//HashChain // insecure
)

type MsgHeader struct {
	Network ngtypes.Network

	ID        []byte
	Type      MsgType
	Timestamp uint64
	PeerKey   []byte
	Sign      []byte
}

type Message struct {
	Header  *MsgHeader
	Payload []byte
}

// NewHeader is a helper method: generate message data shared between all node's p2p protocols
func NewHeader(host core.Host, network ngtypes.Network, msgID []byte, msgType MsgType) *MsgHeader {
	peerKey, err := host.Peerstore().PubKey(host.ID()).Bytes()
	if err != nil {
		panic("Failed to get public key for sender from local peer store.")
	}

	return &MsgHeader{
		Network:   network,
		ID:        msgID,
		Type:      msgType,
		Timestamp: uint64(time.Now().Unix()),
		PeerKey:   peerKey,
		Sign:      nil,
	}
}
