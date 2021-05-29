package wired

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/message"
)

func (w *Wired) SendGetSheet(peerID peer.ID, checkpointHeight uint64, checkpointHash []byte) (id []byte, stream network.Stream, err error) {
	payload, err := proto.Marshal(&message.GetSheetPayload{
		CheckpointHeight: checkpointHeight,
		CheckpointHash:   checkpointHash,
	})
	if err != nil {
		err = fmt.Errorf("failed to sign pb data: %s", err)
		log.Debug(err)
		return nil, nil, err
	}

	id, _ = uuid.New().MarshalBinary()

	// create message data
	req := &message.Message{
		Header:  NewHeader(w.host, w.network, id, message.MessageType_GETCHAIN),
		Payload: payload,
	}

	// sign the data
	signature, err := Signature(w.host, req)
	if err != nil {
		err = fmt.Errorf("failed to sign pb data: %s", err)
		log.Debug(err)
		return nil, nil, err
	}

	// add the signature to the message
	req.Header.Sign = signature

	stream, err = Send(w.host, w.protocolID, peerID, req)
	if err != nil {
		log.Debug(err)
		return nil, nil, err
	}

	log.Debugf("getsheet to: %s was sent. Message Id: %x, request sheet @%d: %x", peerID, req.Header.MessageId, checkpointHeight, checkpointHash)

	return req.Header.MessageId, stream, nil
}

func (w *Wired) onGetSheet(stream network.Stream, msg *message.Message) {
	log.Debugf("Received getsheet request from %s.", stream.Conn().RemotePeer())

	getSheetPayload := &message.GetSheetPayload{}

	err := proto.Unmarshal(msg.Payload, getSheetPayload)
	if err != nil {
		w.sendReject(msg.Header.MessageId, stream, err)
		return
	}

	log.Debugf("getsheet requests sheet@%d: %x", getSheetPayload.CheckpointHeight, getSheetPayload.CheckpointHash)

	sheet := w.chain.GetSnapshot(getSheetPayload.CheckpointHeight, getSheetPayload.CheckpointHash)
	if sheet == nil {
		err = fmt.Errorf("cannot find the snapshot on such height")
		w.sendReject(msg.Header.MessageId, stream, err)
	}

	w.sendSheet(msg.Header.MessageId, stream, sheet)
}
