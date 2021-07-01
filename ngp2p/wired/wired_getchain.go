package wired

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/c0mm4nd/rlp"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (w *Wired) SendGetChain(peerID peer.ID, from [][]byte, to []byte) (id []byte, stream network.Stream, err error) {
	// avoid nil
	if to == nil {
		to = ngtypes.GetEmptyHash()
	}

	payload, err := rlp.EncodeToBytes(&GetChainPayload{
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
	req := &Message{
		Header:  NewHeader(w.host, w.network, id, GetChainMsg),
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

	log.Debugf("getchain to: %s was sent. Message Id: %x, request blocks: %s to %s", peerID, req.Header.ID, fmtFromField(from), string(to))

	return req.Header.ID, stream, nil
}

// RULE:
// request [[from a...from b]...to]
// if to is nil, converging mode on
//
// converging mode:
// 1. check all hashes in db and try to find existing one(samepoint)
// 2. if none, return nil
// 3. if index==0,return everything back
//
// sync mode:
// parse request to [[peerHeight], to]
// return [peerHeight+1, ..., to].
func (w *Wired) onGetChain(stream network.Stream, msg *Message) {
	log.Debugf("Received getchain request from %s.", stream.Conn().RemotePeer())

	getChainPayload := &GetChainPayload{}

	err := rlp.DecodeBytes(msg.Payload, getChainPayload)
	if err != nil {
		w.sendReject(msg.Header.ID, stream, err)
		return
	}

	blocks := make([]*ngtypes.Block, 0, defaults.MaxBlocks)

	if getChainPayload.From == nil || len(getChainPayload.From) == 0 && len(getChainPayload.To) == 16 {
		// fetching mode
		from := binary.LittleEndian.Uint64(getChainPayload.To[0:8])
		to := binary.LittleEndian.Uint64(getChainPayload.To[8:16])
		for blockHeight := from; blockHeight <= to; blockHeight++ {
			cur, err := w.chain.GetBlockByHeight(blockHeight)
			if err != nil {
				err := fmt.Errorf("chain lacks block@%d: %s", blockHeight, err)
				log.Error(err)
				w.sendReject(msg.Header.ID, stream, err)
				return
			}

			blocks = append(blocks, cur)
		}

		w.sendChain(msg.Header.ID, stream, blocks...)
		return
	}

	log.Debugf("getchain requests from %x to %x", getChainPayload.From[0], getChainPayload.To)

	// init cur
	cur, err := w.chain.GetBlockByHash(getChainPayload.From[0])
	if err != nil {
		err = fmt.Errorf("cannot get block by hash %x: %s", getChainPayload.From[0], err)
		log.Error(err)
		w.sendReject(msg.Header.ID, stream, err)
		return
	}

	// run converging mode
	if len(getChainPayload.To) == 16 {
		var samepointIndex int
		// do hashes check first
		for samepointIndex < len(getChainPayload.From) {
			_, err := w.chain.GetBlockByHash(getChainPayload.From[samepointIndex])
			if err == nil {
				// err == nil means found the samepoint
				break
			}

			samepointIndex++
		}

		if samepointIndex == len(getChainPayload.From) {
			// not found samepoint, return nil
			from := binary.LittleEndian.Uint64(getChainPayload.To[0:8])
			to := binary.LittleEndian.Uint64(getChainPayload.To[8:16])
			for blockHeight := from; blockHeight <= to; blockHeight++ {
				cur, err = w.chain.GetBlockByHeight(blockHeight)
				if err != nil {
					err := fmt.Errorf("chain lacks block@%d: %s", blockHeight, err)
					log.Debug(err)
					w.sendReject(msg.Header.ID, stream, err)
					return
				}

				blocks = append(blocks, cur)
			}

			w.sendChain(msg.Header.ID, stream, blocks...)
			return
		}

		// not include this point
		cur, err = w.chain.GetBlockByHash(getChainPayload.From[samepointIndex])
		if err != nil {
			w.sendReject(msg.Header.ID, stream, err)
			return
		}

		for i := 0; i < len(getChainPayload.From)-1-samepointIndex; i++ {
			blockHeight := cur.Header.Height + 1
			cur, err = w.chain.GetBlockByHeight(blockHeight)
			if err != nil {
				err := fmt.Errorf("chain lacks block@%d: %s", blockHeight, err)
				log.Debug(err)
				w.sendReject(msg.Header.ID, stream, err)
				return
			}

			blocks = append(blocks, cur)
		}
	} else if len(getChainPayload.To) == 32 {
		// fetch mode
		for i := 0; i < defaults.MaxBlocks; i++ {
			// never reach To
			if bytes.Equal(cur.GetHash(), getChainPayload.To) {
				break
			}

			nextHeight := cur.Header.Height + 1
			cur, err = w.chain.GetBlockByHeight(nextHeight)
			if err != nil {
				log.Debugf("local chain lacks block@%d: %s", nextHeight, err)
				break
			}

			blocks = append(blocks, cur)
		}
	}

	w.sendChain(msg.Header.ID, stream, blocks...)
}

func fmtFromField(from [][]byte) string {
	hashes := make([]string, len(from))
	for i := 0; i < len(from); i++ {
		hashes[i] = hex.EncodeToString(from[i])
	}

	json, _ := utils.JSON.MarshalToString(hashes)
	return json
}
