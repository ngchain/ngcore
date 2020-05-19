package ngp2p

import (
	"github.com/libp2p/go-libp2p-core/network"
)

// reject will reply reject message to remote node.
func (w *wiredProtocol) reject(uuid []byte, stream network.Stream, err error) bool {
	log.Debugf("Sending reject to %s. Message id: %x...", stream.Conn().RemotePeer(), uuid)

	resp := &Message{
		Header:  w.node.NewHeader(uuid, MessageType_REJECT),
		Payload: []byte(err.Error()),
	}

	// sign the data
	signature, err := signMessage(w.node.PrivKey(), resp)
	if err != nil {
		log.Errorf("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	err = w.node.replyToStream(stream, resp)
	if err != nil {
		log.Debugf("chain to: %s was sent. Message Id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)
		return false
	}

	log.Debugf("chain to: %s was sent. Message Id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)

	return true
}
