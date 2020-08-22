package wired

import (
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngchain/ngcore/ngp2p/message"
)

// reject will reply reject message to remote node.
func (w *Wired) reject(uuid []byte, stream network.Stream, err error) bool {
	log.Debugf("sending reject to %s with message id: %x...", stream.Conn().RemotePeer(), uuid)

	resp := &message.Message{
		Header:  message.NewHeader(w.host, uuid, message.MessageType_REJECT),
		Payload: []byte(err.Error()),
	}

	// sign the data
	signature, err := message.Signature(w.host, resp)
	if err != nil {
		log.Debugf("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	err = message.ReplyToStream(stream, resp)
	if err != nil {
		log.Debugf("sent chain to: %s was with message Id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)
		return false
	}

	log.Debugf("sent chain to: %s with message Id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)

	return true
}
