package ngp2p

import (
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/ngchain/ngcore/ngchain"

	"github.com/ngchain/ngcore/utils"
)

func (w *wiredProtocol) Ping(peerID peer.ID, origin, latest uint64, checkpointHash []byte, checkpointActualDiff []byte) (id []byte,
	stream network.Stream) {
	payload, err := utils.Proto.Marshal(&PingPayload{
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
	req := &Message{
		Header:  w.node.NewHeader(id, MessageType_PING),
		Payload: payload,
	}

	// sign the data
	signature, err := signMessage(w.node.PrivKey(), req)
	if err != nil {
		log.Debugf("failed to sign pb data")
		return nil, nil
	}

	// add the signature to the message
	req.Header.Sign = signature

	log.Debugf("Sent ping to: %s was sent. Message Id: %x", peerID, req.Header.MessageId)

	stream, err = w.node.sendProtoMessage(peerID, req)
	if err != nil {
		log.Debugf("failed sending ping to: %s: %s", peerID, err)
		return nil, nil
	}

	return req.Header.MessageId, stream
}

// remote peer requests handler
func (w *wiredProtocol) onPing(stream network.Stream, msg *Message) {
	ping := &PingPayload{}

	err := utils.Proto.Unmarshal(msg.Payload, ping)
	if err != nil {
		w.reject(msg.Header.MessageId, stream, err)
		return
	}

	log.Debugf("Received ping request from %s.", stream.Conn().RemotePeer())

	// send pong

	origin := ngchain.GetOriginBlock()
	latest := ngchain.GetLatestBlock()
	checkpoint := ngchain.GetLatestCheckpoint()
	w.pong(msg.Header.MessageId, stream, origin.GetHeight(), latest.GetHeight(), checkpoint.Hash(), checkpoint.GetActualDiff().Bytes())
}
