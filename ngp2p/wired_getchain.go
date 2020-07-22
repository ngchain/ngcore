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
		log.Debugf("failed to send getChain: from is nil")

		return nil, nil
	}

	payload, err := utils.Proto.Marshal(&GetChainPayload{
		From: from,
		To:   to,
	})
	if err != nil {
		log.Debugf("failed to sign pb data")
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
		log.Debugf("failed to sign pb data")
		return nil, nil
	}

	// add the signature to the message
	req.Header.Sign = signature

	stream, err = w.node.sendProtoMessage(peerID, req)
	if err != nil {
		log.Debug(err)
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

	var lastFromHash []byte

	if len(getChainPayload.GetFrom()) == 0 {
		lastFromHash = ngtypes.GetGenesisBlockHash()
	} else {
		// do hashes check first
		for i := 0; i < len(getChainPayload.GetFrom()); i++ {
			_, err := storage.GetChain().GetBlockByHash(getChainPayload.GetFrom()[i])

			if err != nil {
				break
			}

			// finally fetch blocks since the last common block hash
			lastFromHash = getChainPayload.GetFrom()[i]
		}
	}

	log.Debugf("Received getchain request from %s. Requested %x to %x", stream.Conn().RemotePeer(), lastFromHash, getChainPayload.GetTo())

	// dont input the same block
	cur, err := storage.GetChain().GetBlockByHash(lastFromHash)
	if err != nil {
		w.reject(msg.Header.MessageId, stream, err)
		return
	}

	blocks := make([]*ngtypes.Block, 0)
	for i := 0; i < MaxBlocks; i++ {
		if bytes.Equal(cur.Hash(), getChainPayload.GetTo()) {
			break
		}

		nextHeight := cur.GetHeight() + 1
		cur, err = storage.GetChain().GetBlockByHeight(nextHeight)
		if err != nil {
			log.Debugf("local chain is missing block@%d: %s", nextHeight, err)
			break
		}

		blocks = append(blocks, cur)
	}

	w.chain(msg.Header.MessageId, stream, blocks...)
}
