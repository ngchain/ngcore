package ngp2p

import (
	"io/ioutil"

	"github.com/libp2p/go-libp2p-core/network"
	"google.golang.org/protobuf/proto"
)

// reject will reply reject message to remote node.
func (w *wiredProtocol) reject(uuid []byte, stream network.Stream, err error) bool {
	log.Debugf("Sending reject to %s. Message id: %s...", stream.Conn().RemotePeer(), uuid)

	resp := &Message{
		Header:  w.node.NewHeader(uuid, MessageType_REJECT),
		Payload: nil,
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
		log.Debugf("chain to: %s was sent. Message Id: %s", stream.Conn().RemotePeer(), resp.Header.MessageId)
		return false
	}

	log.Debugf("chain to: %s was sent. Message Id: %s", stream.Conn().RemotePeer(), resp.Header.MessageId)

	return true
}

// remote reject handler.
func (w *wiredProtocol) onReject(s network.Stream) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		log.Error(err)

		_ = s.Reset()

		return
	}

	remotePeerID := s.Conn().RemotePeer()
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

	log.Debugf("Received reject from %s. Message id:%s. Message: %s.", remotePeerID, data.Header.MessageId, data.Payload)
}
