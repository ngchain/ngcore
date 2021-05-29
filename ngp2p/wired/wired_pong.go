package wired

import (
	"github.com/libp2p/go-libp2p-core/network"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/message"
)

func (w *Wired) sendPong(uuid []byte, stream network.Stream, origin, latest uint64, checkpointHash, checkpointActualDiff []byte) bool {
	log.Debugf("sending pong to %s. Message id: %x...", stream.Conn().RemotePeer(), uuid)

	pongPayload := &message.PongPayload{
		Origin:               origin,
		Latest:               latest,
		CheckpointHash:       checkpointHash,
		CheckpointActualDiff: checkpointActualDiff,
	}

	rawPayload, err := proto.Marshal(pongPayload)
	if err != nil {
		return false
	}

	resp := &message.Message{
		Header:  NewHeader(w.host, w.network, uuid, message.MessageType_PONG),
		Payload: rawPayload,
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
		log.Debugf("failed sending pong to: %s: %s", stream.Conn().RemotePeer(), err)
		return false
	}

	log.Debugf("sent pong to: %s with message id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)

	return true
}

// DecodePongPayload unmarshal the raw and return the *message.PongPayload.
func DecodePongPayload(rawPayload []byte) (*message.PongPayload, error) {
	pongPayload := &message.PongPayload{}

	err := proto.Unmarshal(rawPayload, pongPayload)
	if err != nil {
		return nil, err
	}

	return pongPayload, nil
}
