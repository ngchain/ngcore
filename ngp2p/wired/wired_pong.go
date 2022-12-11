package wired

import (
	"github.com/c0mm4nd/rlp"
	"github.com/libp2p/go-libp2p/core/network"
)

func (w *Wired) sendPong(uuid []byte, stream network.Stream, origin, latest uint64, checkpointHash, checkpointActualDiff []byte) bool {
	log.Debugf("sending pong to %s. Message id: %x...", stream.Conn().RemotePeer(), uuid)

	pongPayload := &StatusPayload{
		Origin:         origin,
		Latest:         latest,
		CheckpointHash: checkpointHash,
		CheckpointDiff: checkpointActualDiff,
	}

	rawPayload, err := rlp.EncodeToBytes(pongPayload)
	if err != nil {
		return false
	}

	resp := &Message{
		Header:  NewHeader(w.host, w.network, uuid, PongMsg),
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

	log.Debugf("sent pong to: %s with message id: %x", stream.Conn().RemotePeer(), resp.Header.ID)

	return true
}

// DecodePongPayload unmarshal the raw and return the *message.PongPayload.
func DecodePongPayload(rawPayload []byte) (*StatusPayload, error) {
	var pongPayload StatusPayload

	err := rlp.DecodeBytes(rawPayload, &pongPayload)
	if err != nil {
		return nil, err
	}

	return &pongPayload, nil
}
