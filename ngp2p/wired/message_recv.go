package wired

import (
	"bytes"

	"github.com/c0mm4nd/rlp"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-msgio"
	"github.com/pkg/errors"
)

var (
	ErrMsgMalformed      = errors.New("malformed message")
	ErrMsgIDInvalid      = errors.New("message id is invalid")
	ErrMsgTypeInvalid    = errors.New("message type is invalid")
	ErrMsgSignInvalid    = errors.New("message sign is invalid")
	ErrMsgPayloadInvalid = errors.New("message payload is invalid")
)

// ReceiveReply will receive the correct reply message from the stream.
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
		return nil, errors.Wrap(ErrMsgMalformed, "response doesnt have msg header")
	}

	if msg.Header.Type == InvalidMsg {
		return nil, errors.Wrap(ErrMsgTypeInvalid, "invalid message type")
	}

	if !bytes.Equal(msg.Header.ID, uuid) {
		return nil, errors.Wrap(ErrMsgIDInvalid, "invalid message id")
	}

	if !Verify(stream.Conn().RemotePeer(), &msg) {
		return nil, errors.Wrap(ErrMsgSignInvalid, "failed to verify the sign of message")
	}

	return &msg, nil
}
