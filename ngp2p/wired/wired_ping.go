package wired

import (
	"github.com/c0mm4nd/rlp"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

func (w *Wired) SendPing(peerID peer.ID, origin, latest uint64, checkpointHash, checkpointActualDiff []byte) (id []byte,
	stream network.Stream) {
	payload, err := rlp.EncodeToBytes(&StatusPayload{
		Origin:         origin,
		Latest:         latest,
		CheckpointHash: checkpointHash,
		CheckpointDiff: checkpointActualDiff,
	})
	if err != nil {
		log.Debugf("failed to sign pb data")
		return nil, nil
	}

	id, _ = uuid.New().MarshalBinary()

	// create message data
	req := &Message{
		Header:  NewHeader(w.host, w.network, id, PingMsg),
		Payload: payload,
	}

	// sign the data
	signature, err := Signature(w.host, req)
	if err != nil {
		log.Debugf("failed to sign pb data, %s", err)
		return nil, nil
	}

	// add the signature to the message
	req.Header.Sign = signature

	log.Debugf("Sent ping to: %s was sent. Message Id: %x", peerID, req.Header.ID)

	stream, err = Send(w.host, w.protocolID, peerID, req)
	if err != nil {
		log.Debugf("failed sending ping to: %s: %s", peerID, err)
		return nil, nil
	}

	return req.Header.ID, stream
}

// remote peer requests handler.
func (w *Wired) onPing(stream network.Stream, msg *Message) {
	log.Debugf("Received remoteStatus request from %s.", stream.Conn().RemotePeer())

	var remoteStatus StatusPayload
	err := rlp.DecodeBytes(msg.Payload, &remoteStatus)
	if err != nil {
		w.sendReject(msg.Header.ID, stream, err)
		return
	}

	// send sendPong
	origin := w.chain.GetOriginBlock()
	latest := w.chain.GetLatestBlock()
	checkpoint := w.chain.GetLatestCheckpoint()
	w.sendPong(msg.Header.ID, stream, origin.GetHeight(), latest.GetHeight(), checkpoint.GetHash(), checkpoint.GetActualDiff().Bytes())
}
