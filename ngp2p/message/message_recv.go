package message

import (
	"bytes"
	"fmt"
	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-msgio"

	"github.com/ngchain/ngcore/utils"
)

// ReceiveReply will receive the correct reply message from the stream
func ReceiveReply(uuid []byte, stream network.Stream) (*Message, error) {
	r := msgio.NewReader(stream)
	raw, err := r.ReadMsg()
	if err != nil {
		return nil, err
	}

	err = helpers.FullClose(stream)
	if err != nil {
		return nil, err
	}

	msg := &Message{}

	err = utils.Proto.Unmarshal(raw, msg)
	if err != nil {
		return nil, err
	}

	if msg.Header == nil {
		return nil, fmt.Errorf("malformed response")
	}

	if msg.Header.MessageType == MessageType_INVALID {
		return nil, fmt.Errorf("invalid message type")
	}

	if !bytes.Equal(msg.Header.MessageId, uuid) {
		return nil, fmt.Errorf("invalid message id")
	}

	if !Verify(stream.Conn().RemotePeer(), msg) {
		return nil, fmt.Errorf("failed to verify the sign of message")
	}

	return msg, nil
}
