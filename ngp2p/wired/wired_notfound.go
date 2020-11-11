package wired

import (
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngchain/ngcore/ngp2p/message"
)

// sendNotFound will reply sendNotFound message to remote node.
func (w *Wired) sendNotFound(uuid []byte, stream network.Stream, blockHash []byte) bool {
	log.Debugf("sending notfound to %s with message id: %x...", stream.Conn().RemotePeer(), uuid)

	resp := &message.Message{
		Header:  NewHeader(w.host, w.network, uuid, message.MessageType_NOTFOUND),
		Payload: blockHash,
	}

	// sign the data
	signature, err := Signature(w.host, resp)
	if err != nil {
		log.Errorf("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	err = Reply(stream, resp)
	if err != nil {
		log.Debugf("sent notfound to: %s with message Id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)
		return false
	}

	log.Debugf("sent notfound to: %s with message Id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)

	return true
}
