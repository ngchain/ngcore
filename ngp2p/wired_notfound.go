package ngp2p

import (
	"io/ioutil"

	"github.com/libp2p/go-libp2p-core/network"
	"google.golang.org/protobuf/proto"
)

// notFound will reply notFound message to remote node.
func (w *wiredProtocol) notFound(uuid []byte, stream network.Stream, blockHash []byte) bool {
	log.Debugf("Sending notfound to %s. Message id: %s...", stream.Conn().RemotePeer(), uuid)

	resp := &Message{
		Header:  w.node.NewHeader(uuid, MessageType_NOTFOUND),
		Payload: blockHash,
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
		log.Debugf("notfound to: %s was sent. Message Id: %s", stream.Conn().RemotePeer(), resp.Header.MessageId)
		return false
	}

	log.Debugf("notfound to: %s was sent. Message Id: %s", stream.Conn().RemotePeer(), resp.Header.MessageId)

	return true
}

// onNotFound is a remote notfound handler. When received a notfound, local node is running on a wrong chain
func (w *wiredProtocol) onNotFound(s network.Stream) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		log.Error(err)

		_ = s.Reset()

		return
	}

	_ = s.Close()

	// unmarshal it
	var data = &Message{}

	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Error(err)

		return
	}

	if !verifyMessage(s.Conn().RemotePeer(), data) {
		log.Errorf("Failed to authenticate message")

		return
	}

	remoteID := s.Conn().RemotePeer()
	_ = s.Close()

	log.Debugf("Received notfound from %s. Message id:%s. Message: %s.", remoteID, data.Header.MessageId, data.Payload)
}
