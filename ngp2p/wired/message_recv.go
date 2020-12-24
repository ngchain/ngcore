package wired

import (
	"bytes"
	"fmt"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-msgio"

	"github.com/ngchain/ngcore/ngp2p/message"

	"github.com/ngchain/ngcore/utils"
)

// ReceiveReply will receive the correct reply message from the stream
func ReceiveReply(uuid []byte, stream network.Stream) (*message.Message, error) {
	r := msgio.NewReader(stream)
	raw, err := r.ReadMsg()
	if err != nil {
		return nil, err
	}

	err = stream.Close()
	if err != nil {
		return nil, err
	}

	msg := &message.Message{}

	err = utils.Proto.Unmarshal(raw, msg)
	if err != nil {
		return nil, err
	}

	if msg.Header == nil {
		return nil, fmt.Errorf("malformed response")
	}

	if msg.Header.MessageType == message.MessageType_INVALID {
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
