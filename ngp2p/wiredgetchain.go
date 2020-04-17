package ngp2p

import (
	"io/ioutil"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (w *wired) getChain(peerID peer.ID, from uint64, to uint64) bool {
	if from < 0 {
		from = 0
	}

	if to < from {
		log.Errorf("from %d is larger than to %d!", from, to)
		return false
	}

	if to == from {
		return false
	}

	if to-from > ngtypes.BlockCheckRound {
		to = from + ngtypes.BlockCheckRound
	}

	payload, err := utils.Proto.Marshal(&pb.GetChainPayload{
		From: from,
		To:   to,
	})
	if err != nil {
		log.Error("failed to sign pb data")
		return false
	}

	// create message data
	req := &pb.Message{
		Header:  w.node.NewHeader(uuid.New().String()),
		Payload: payload,
	}

	// sign the data
	signature, err := w.node.signMessage(req)
	if err != nil {
		log.Error("failed to sign pb data")
		return false
	}

	// add the signature to the message
	req.Header.Sign = signature

	ok := w.node.sendProtoMessage(peerID, getChainMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	w.requests.Store(req.Header.Uuid, req)
	log.Debugf("getchain to: %s was sent. Message Id: %s, request height: %d to %d", peerID, req.Header.Uuid, from, to)
	return true
}

func (w *wired) onGetChain(s network.Stream) {
	// get request data
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		_ = s.Reset()
		log.Error(err)
		return
	}

	// unmarshal it
	var data = &pb.Message{}
	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Error(err)
		return
	}

	if !w.node.authenticateMessage(s.Conn().RemotePeer(), data) {
		log.Errorf("Failed to authenticate message")
		return
	}

	var payload = &pb.GetChainPayload{}
	err = proto.Unmarshal(data.Payload, payload)
	if err != nil {
		log.Error(err)
		return
	}

	remoteID := s.Conn().RemotePeer()
	_ = s.Close()

	log.Debugf("Received getchain request from %s. Requested %d to %d", remoteID, payload.From, payload.To)

	if payload.From > payload.To || payload.To-payload.From > 200 {
		return
	}

	var blocks = make([]*ngtypes.Block, 0, ngtypes.BlockCheckRound)
	for i := payload.From; i <= payload.To; i++ {
		b, err := w.node.consensus.GetBlockByHeight(i)
		if err != nil {
			log.Errorf("missing block@%d: %s", i, err)
			break
		}

		if b == nil {
			log.Errorf("missing block@%d", i)
			break
		}

		blocks = append(blocks, b)
	}

	go w.chain(remoteID, data.Header.Uuid, blocks...)
}
