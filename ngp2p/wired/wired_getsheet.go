package wired

import (
	"github.com/c0mm4nd/rlp"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/pkg/errors"
)

func (w *Wired) SendGetSheet(peerID peer.ID, checkpointHeight uint64, checkpointHash []byte) (id []byte, stream network.Stream, err error) {
	payload, err := rlp.EncodeToBytes(&GetSheetPayload{
		Height: checkpointHeight,
		Hash:   checkpointHash,
	})
	if err != nil {
		err = errors.Wrapf(err, "failed to encode data into rlp")
		log.Debug(err)
		return nil, nil, err
	}

	id, _ = uuid.New().MarshalBinary()

	// create message data
	req := &Message{
		Header:  NewHeader(w.host, w.network, id, GetChainMsg),
		Payload: payload,
	}

	// sign the data
	signature, err := Signature(w.host, req)
	if err != nil {
		err = errors.Wrap(err, "failed to sign pb data")
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

	log.Debugf("getsheet to: %s was sent. Message Id: %x, request sheet @%d: %x", peerID, req.Header.ID, checkpointHeight, checkpointHash)

	return req.Header.ID, stream, nil
}

func (w *Wired) onGetSheet(stream network.Stream, msg *Message) {
	log.Debugf("Received getsheet request from %s.", stream.Conn().RemotePeer())

	var getSheetPayload GetSheetPayload

	err := rlp.DecodeBytes(msg.Payload, &getSheetPayload)
	if err != nil {
		w.sendReject(msg.Header.ID, stream, err)
		return
	}

	log.Debugf("getsheet requests sheet@%d: %x", getSheetPayload.Height, getSheetPayload.Hash)

	sheet := w.chain.GetSnapshot(getSheetPayload.Height, getSheetPayload.Hash)
	if sheet == nil {
		w.sendReject(msg.Header.ID, stream, ngstate.ErrSnapshotNofFound)
	}

	w.sendSheet(msg.Header.ID, stream, sheet)
}
