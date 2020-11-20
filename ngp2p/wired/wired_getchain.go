package wired

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngp2p/message"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (w *Wired) SendGetChain(peerID peer.ID, from [][]byte, to []byte) (id []byte, stream network.Stream, err error) {
	if len(from) == 0 {
		err := fmt.Errorf("getchain's from is nil")
		log.Debug(err)

		return nil, nil, err
	}

	// avoid nil hash
	if to == nil {
		to = ngtypes.GetEmptyHash()
	}

	payload, err := utils.Proto.Marshal(&message.GetChainPayload{
		From: from,
		To:   to,
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

	log.Debugf("getchain to: %s was sent. Message Id: %x, request blocks: %s to %s", peerID, req.Header.MessageId, fmtFromField(from), string(to))

	return req.Header.MessageId, stream, nil
}

func (w *Wired) onGetChain(stream network.Stream, msg *message.Message) {
	log.Debugf("Received getchain request from %s.", stream.Conn().RemotePeer())

	getChainPayload := &message.GetChainPayload{}

	err := utils.Proto.Unmarshal(msg.Payload, getChainPayload)
	if err != nil {
		w.sendReject(msg.Header.MessageId, stream, err)
		return
	}

	if len(getChainPayload.GetFrom()) == 0 {
		w.sendReject(msg.Header.MessageId, stream, err)
		return
	}

	var forkpointIndex int
	// do hashes check first
	for forkpointIndex < len(getChainPayload.GetFrom()) {
		_, err := w.chain.GetBlockByHash(getChainPayload.GetFrom()[forkpointIndex])
		if err != nil {
			// failed to get the block from local chain means
			// there is a fork since this block(aka fork point)
			// and its prevBlock is the last same one
			break
		}

		forkpointIndex++
		if forkpointIndex == len(getChainPayload.GetFrom()) {
			forkpointIndex = -1 // not found forkpoint, return all
		}
	}

	lastFromHashIndex := forkpointIndex + 1

	log.Debugf("getchain requests from %x to %x", getChainPayload.GetFrom()[lastFromHashIndex], getChainPayload.GetTo())

	cur, err := w.chain.GetBlockByHash(getChainPayload.GetFrom()[lastFromHashIndex])
	if err != nil {
		w.sendReject(msg.Header.MessageId, stream, err)
		return
	}

	blocks := make([]*ngtypes.Block, 0, defaults.MaxBlocks)
	for i := 0; i < defaults.MaxBlocks; i++ {
		if bytes.Equal(cur.Hash(), getChainPayload.GetTo()) {
			break
		}

		nextHeight := cur.GetHeight() + 1
		cur, err = w.chain.GetBlockByHeight(nextHeight)
		if err != nil {
			log.Debugf("local chain lacks block@%d: %s", nextHeight, err)
			break
		}

		blocks = append(blocks, cur)
	}

	w.sendChain(msg.Header.MessageId, stream, blocks...)
}

func fmtFromField(from [][]byte) string {
	hashes := make([]string, len(from))
	for i := 0; i < len(from); i++ {
		hashes[i] = hex.EncodeToString(from[i])
	}

	json, _ := utils.JSON.MarshalToString(hashes)
	return json
}
