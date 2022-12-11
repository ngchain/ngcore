package wired

import (
	"github.com/libp2p/go-libp2p/core/network"
)

// sendReject will reply sendReject message to remote node.
func (w *Wired) sendReject(uuid []byte, stream network.Stream, err error) bool {
	log.Debugf("sending sendReject to %s with message id: %x...", stream.Conn().RemotePeer(), uuid)

	resp := &Message{
		Header:  NewHeader(w.host, w.network, uuid, RejectMsg),
		Payload: []byte(err.Error()),
	}

	// sign the data
	signature, err := Signature(w.host, resp)
	if err != nil {
		log.Debugf("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	err = Reply(stream, resp)
	if err != nil {
		log.Debugf("sent sendChain to: %s was with message Id: %x", stream.Conn().RemotePeer(), resp.Header.ID)
		return false
	}

	log.Debugf("sent sendChain to: %s with message Id: %x", stream.Conn().RemotePeer(), resp.Header.ID)

	return true
}
