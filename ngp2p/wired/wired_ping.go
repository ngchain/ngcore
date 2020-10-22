package wired

import (
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/ngchain/ngcore/ngp2p/message"

	"github.com/ngchain/ngcore/utils"
)

func (w *Wired) SendPing(peerID peer.ID, origin, latest uint64, checkpointHash, checkpointActualDiff []byte) (id []byte,
	stream network.Stream) {
	payload, err := utils.Proto.Marshal(&message.PingPayload{
		Origin:               origin,
		Latest:               latest,
		CheckpointHash:       checkpointHash,
		CheckpointActualDiff: checkpointActualDiff,
	})
	if err != nil {
		log.Debugf("failed to sign pb data")
		return nil, nil
	}

	id, _ = uuid.New().MarshalBinary()

	// create message data
	req := &message.Message{
		Header:  NewHeader(w.host, w.network, id, message.MessageType_PING),
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

	log.Debugf("Sent ping to: %s was sent. Message Id: %x", peerID, req.Header.MessageId)

	stream, err = Send(w.host, peerID, req)
	if err != nil {
		log.Debugf("failed sending ping to: %s: %s", peerID, err)
		return nil, nil
	}

	return req.Header.MessageId, stream
}

// remote peer requests handler
func (w *Wired) onPing(stream network.Stream, msg *message.Message) {
	log.Debugf("Received ping request from %s.", stream.Conn().RemotePeer())
	ping := &message.PingPayload{}

	err := utils.Proto.Unmarshal(msg.Payload, ping)
	if err != nil {
		w.sendReject(msg.Header.MessageId, stream, err)
		return
	}

	// send sendPong
	origin := w.chain.GetOriginBlock()
	latest := w.chain.GetLatestBlock()
	checkpoint := w.chain.GetLatestCheckpoint()
	w.sendPong(msg.Header.MessageId, stream, origin.GetHeight(), latest.GetHeight(), checkpoint.Hash(), checkpoint.GetActualDiff().Bytes())
}
