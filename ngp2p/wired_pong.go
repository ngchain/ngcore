package ngp2p

import (
	"github.com/libp2p/go-libp2p-core/network"

	"github.com/ngchain/ngcore/utils"
)

func (w *wiredProtocol) pong(uuid []byte, stream network.Stream) bool {
	log.Debugf("Sending pong to %s. Message id: %x...", stream.Conn().RemotePeer(), uuid)

	pongPayload := &PongPayload{
		Origin:         0,
		Latest:         0,
		CheckpointHash: nil,
	}

	rawPayload, err := utils.Proto.Marshal(pongPayload)
	if err != nil {
		return false
	}

	resp := &Message{
		Header:  w.node.NewHeader(uuid, MessageType_PONG),
		Payload: rawPayload,
	}

	// sign the data
	signature, err := signMessage(w.node.PrivKey(), resp)
	if err != nil {
		log.Error("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	err = w.node.replyToStream(stream, resp)
	if err != nil {
		log.Debugf("pong to: %s was sent. Message Id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)
		return false
	}

	log.Debugf("pong to: %s was sent. Message Id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)

	return true
}

// DecodePongPayload unmarshal the raw and return the *pb.PongPayload.
func DecodePongPayload(rawPayload []byte) (*PongPayload, error) {
	pongPayload := &PongPayload{}
	err := utils.Proto.Unmarshal(rawPayload, pongPayload)
	if err != nil {
		return nil, err
	}

	return pongPayload, nil
}
