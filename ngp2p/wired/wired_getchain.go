package wired

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"

	"github.com/c0mm4nd/rlp"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"

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
		err = errors.Wrap(err, "failed to encode data into rlp")
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

	var getChainPayload GetChainPayload

	err := rlp.DecodeBytes(msg.Payload, &getChainPayload)
	if err != nil {
		w.sendReject(msg.Header.ID, stream, err)
		return
	}

	blocks := make([]*ngtypes.FullBlock, 0, defaults.MaxBlocks)

	// validate the request shape up front: To is either a 16-byte
	// from‖to height range or a 32-byte hash. Without this a malformed
	// payload from ANY peer (empty From with a 32-byte To, or a short
	// To) indexes past From[0] / To[0:8] and panics the stream handler —
	// a remote node crash
	if len(getChainPayload.To) != 16 && len(getChainPayload.To) != 32 {
		w.sendReject(msg.Header.ID, stream, errors.Errorf("getchain: To must be 16 or 32 bytes, got %d", len(getChainPayload.To)))
		return
	}

	if len(getChainPayload.From) == 0 {
		// fetching mode: no known hashes, To carries a 16-byte height range
		if len(getChainPayload.To) != 16 {
			w.sendReject(msg.Header.ID, stream, errors.New("getchain: empty From requires a 16-byte height range in To"))
			return
		}
		from := binary.LittleEndian.Uint64(getChainPayload.To[0:8])
		to := binary.LittleEndian.Uint64(getChainPayload.To[8:16])
		for blockHeight := from; blockHeight <= to; blockHeight++ {
			cur, err := w.chain.GetBlockByHeight(blockHeight)
			if err != nil {
				err := errors.Wrapf(err, "chain lacks block@%d", blockHeight)
				log.Error(err)
				w.sendReject(msg.Header.ID, stream, err)
				return
			}

			blocks = append(blocks, cur.(*ngtypes.FullBlock))
		}

		w.sendChain(msg.Header.ID, stream, blocks...)
		return
	}

	log.Debugf("getchain requests from %x to %x", getChainPayload.From[0], getChainPayload.To)

	// run converging mode
	if len(getChainPayload.To) == 16 {
		// NOTE: in converging mode the requester's hashes may be entirely
		// unknown here (that is the point of converging), so nothing is
		// pre-fetched. Requester hashes are ordered by ascending height.
		//
		// Find the HIGHEST-height hash we also have: that is the fork point.
		// Searching from the top means a batch that straddles the fork point
		// returns the divergent suffix (fork+1 ..), not the shared blocks
		// below it (the old lowest-match search returned shared blocks and
		// convergence stalled).
		samepointIndex := -1
		for i := len(getChainPayload.From) - 1; i >= 0; i-- {
			if _, err := w.chain.GetBlockByHash(getChainPayload.From[i]); err == nil {
				samepointIndex = i
				break
			}
		}

		if samepointIndex == -1 {
			// no common block in this batch: the fork point is OLDER than the
			// requester's oldest hash here. Reply with an empty chain so the
			// requester walks further back. (Returning our own blocks for the
			// requested height range — as before — produced a branch whose
			// parent the requester does not have, so it could never attach and
			// a fork deeper than one batch never converged.)
			w.sendChain(msg.Header.ID, stream)
			return
		}

		// the fork point; return OUR chain from fork+1 up to a batch worth,
		// so the reply attaches to a block the requester already stores
		cur, err := w.chain.GetBlockByHash(getChainPayload.From[samepointIndex])
		if err != nil {
			w.sendReject(msg.Header.ID, stream, err)
			return
		}

		for i := 0; i < defaults.MaxBlocks; i++ {
			next, err := w.chain.GetBlockByHeight(cur.(*ngtypes.FullBlock).GetHeight() + 1)
			if err != nil {
				break // reached our tip
			}
			blocks = append(blocks, next.(*ngtypes.FullBlock))
			cur = next
		}
	} else if len(getChainPayload.To) == 32 {
		// fetch mode
		cur, err := w.chain.GetBlockByHash(getChainPayload.From[0])
		if err != nil {
			err = errors.Wrapf(err, "cannot get block by hash %x", getChainPayload.From[0])
			log.Error(err)
			w.sendReject(msg.Header.ID, stream, err)
			return
		}

		for i := 0; i < defaults.MaxBlocks; i++ {
			// never reach To
			if bytes.Equal(cur.GetHash(), getChainPayload.To) {
				break
			}

			nextHeight := cur.(*ngtypes.FullBlock).GetHeight() + 1
			cur, err = w.chain.GetBlockByHeight(nextHeight)
			if err != nil {
				log.Debugf("local chain lacks block@%d: %s", nextHeight, err)
				break
			}

			blocks = append(blocks, cur.(*ngtypes.FullBlock))
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
