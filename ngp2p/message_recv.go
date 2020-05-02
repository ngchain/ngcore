package ngp2p

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/libp2p/go-libp2p-core/network"

	"github.com/ngchain/ngcore/utils"
)

// ReceiveReply will receive the correct reply message from the stream
func ReceiveReply(uuid []byte, stream network.Stream) (*Message, error) {
	raw, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	_ = stream.Close()

	msg := &Message{}
	err = utils.Proto.Unmarshal(raw, msg)
	if err != nil {
		return nil, err
	}

	if msg.Header.MessageType == MessageType_INVALID {
		return nil, fmt.Errorf("invalid message type")
	}

	if !bytes.Equal(msg.Header.MessageId, uuid) {
		return nil, fmt.Errorf("invalid message id")
	}

	if !verifyMessage(stream.Conn().RemotePeer(), msg) {
		return nil, fmt.Errorf("failed to verify the sign of message")
	}

	return msg, nil
}
