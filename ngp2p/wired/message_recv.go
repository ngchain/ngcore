package wired

import (
	"bytes"
	"fmt"
	"github.com/c0mm4nd/rlp"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-msgio"
)

// ReceiveReply will receive the correct reply message from the stream
func ReceiveReply(uuid []byte, stream network.Stream) (*Message, error) {
	r := msgio.NewReader(stream)
	raw, err := r.ReadMsg()
	if err != nil {
		return nil, err
	}

	err = stream.Close()
	if err != nil {
		return nil, err
	}

	var msg Message
	err = rlp.DecodeBytes(raw, msg)
	if err != nil {
		return nil, err
	}

	if msg.Header == nil {
		return nil, fmt.Errorf("malformed response")
	}

	if msg.Header.Type == InvalidMsg {
		return nil, fmt.Errorf("invalid message type")
	}

	if !bytes.Equal(msg.Header.ID, uuid) {
		return nil, fmt.Errorf("invalid message id")
	}

	if !Verify(stream.Conn().RemotePeer(), &msg) {
		return nil, fmt.Errorf("failed to verify the sign of message")
	}

	return &msg, nil
}
