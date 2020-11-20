package wired

import (
	"bytes"
	"encoding/binary"
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

// RULE:
// request [[from a...from b]...to]
// 1. if to is nil, fork mode on
// 2. check all hashes in db and try to find existing one(samepoint)
// 3. if none, return nil
// 3. if index==0,return everything back
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

	log.Debugf("getchain requests from %x to %x", getChainPayload.GetFrom()[0], getChainPayload.GetTo())

	// init cur
	cur, err := w.chain.GetBlockByHash(getChainPayload.GetFrom()[0])
	if err != nil {
		w.sendReject(msg.Header.MessageId, stream, err)
		return
	}

	blocks := make([]*ngtypes.Block, 0, defaults.MaxBlocks)

	// run fork mode
	if len(getChainPayload.GetTo()) == 16 || bytes.Equal(getChainPayload.GetTo(), ngtypes.GetEmptyHash()) {
		var samepointIndex int
		// do hashes check first
		for samepointIndex < len(getChainPayload.GetFrom()) {
			_, err := w.chain.GetBlockByHash(getChainPayload.GetFrom()[samepointIndex])
			if err == nil {
				// err == nil means found the samepoint
				break
			}

			samepointIndex++
		}

		if samepointIndex == len(getChainPayload.GetFrom()) {
			// not found samepoint, return nil
			from := binary.LittleEndian.Uint64(getChainPayload.GetTo()[0:8])
			to := binary.LittleEndian.Uint64(getChainPayload.GetTo()[8:16])
			for blockHeight := from; blockHeight <= to; blockHeight++ {
				cur, err = w.chain.GetBlockByHeight(blockHeight)
				if err != nil {
					err := fmt.Errorf("local chain lacks block@%d: %s", blockHeight, err)
					log.Debug(err)
					w.sendReject(msg.Header.MessageId, stream, err)
					return
				}

				blocks = append(blocks, cur)
			}

			w.sendChain(msg.Header.MessageId, stream, blocks...)
			return
		}

		// not include this point
		cur, err = w.chain.GetBlockByHash(getChainPayload.GetFrom()[samepointIndex])
		if err != nil {
			w.sendReject(msg.Header.MessageId, stream, err)
			return
		}

		for i := 0; i < len(getChainPayload.GetFrom())-samepointIndex; i++ {
			blockHeight := cur.GetHeight() + 1
			cur, err = w.chain.GetBlockByHeight(blockHeight)
			if err != nil {
				err := fmt.Errorf("local chain lacks block@%d: %s", blockHeight, err)
				log.Debug(err)
				w.sendReject(msg.Header.MessageId, stream, err)
				return
			}

			blocks = append(blocks, cur)
		}
	} else if len(getChainPayload.GetTo()) == 32 {
		// fetch mode
		for i := 0; i < defaults.MaxBlocks; i++ {
			// never reach To
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
