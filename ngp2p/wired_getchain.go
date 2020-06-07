package ngp2p

import (
	"bytes"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

func (w *wiredProtocol) GetChain(peerID peer.ID, from [][]byte, to []byte) (id []byte, stream network.Stream) {
	if len(from) == 0 {
		log.Errorf("failed to send getChain: from is nil")

		return nil, nil
	}

	payload, err := utils.Proto.Marshal(&GetChainPayload{
		From: from,
		To:   to,
	})
	if err != nil {
		log.Error("failed to sign pb data")
		return nil, nil
	}

	id, _ = uuid.New().MarshalBinary()

	// create message data
	req := &Message{
		Header:  w.node.NewHeader(id, MessageType_GETCHAIN),
		Payload: payload,
	}

	// sign the data
	signature, err := signMessage(w.node.PrivKey(), req)
	if err != nil {
		log.Error("failed to sign pb data")
		return nil, nil
	}

	// add the signature to the message
	req.Header.Sign = signature

	stream, err = w.node.sendProtoMessage(peerID, req)
	if err != nil {
		log.Error(err)
		return nil, nil
	}

	log.Debugf("getchain to: %s was sent. Message Id: %x, request height: %d to %d", peerID, req.Header.MessageId, from, to)

	return req.Header.MessageId, stream
}

func (w *wiredProtocol) onGetChain(stream network.Stream, msg *Message) {
	getChainPayload := &GetChainPayload{}

	err := utils.Proto.Unmarshal(msg.Payload, getChainPayload)
	if err != nil {
		w.reject(msg.Header.MessageId, stream, err)
		return
	}

	lastFromHash := make([]byte, 32)

	if len(getChainPayload.GetFrom()) == 0 {
		copy(lastFromHash, ngtypes.GetGenesisBlockHash())
	} else {
		copy(lastFromHash, getChainPayload.GetFrom()[len(getChainPayload.GetFrom())-1])
	}

	log.Debugf("Received getchain request from %s. Requested %x to %x", stream.Conn().RemotePeer(), lastFromHash, getChainPayload.GetTo())

	cur, err := storage.GetChain().GetBlockByHash(lastFromHash)
	if err != nil {
		w.reject(msg.Header.MessageId, stream, err)
		return
	}

	blocks := []*ngtypes.Block{cur}
	for i := 0; i < 1000; i++ {
		if bytes.Equal(cur.Hash(), getChainPayload.GetTo()) {
			break
		}

		nextHeight := cur.GetHeight() + 1
		cur, err = storage.GetChain().GetBlockByHeight(nextHeight)
		if err != nil {
			log.Errorf("local chain is missing block@%d: %s", nextHeight, err)
			break
		}

		blocks = append(blocks, cur)
	}

	w.chain(msg.Header.MessageId, stream, blocks...)
}
