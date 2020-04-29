package ngp2p

import (
	"io/ioutil"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/utils"
)

func (w *wiredProtocol) getChain(peerID peer.ID, from [][]byte, to []byte) bool {
	if len(from) == 0 {
		log.Errorf("failed to send getChain: from is nil")

		return false
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

func (w *wiredProtocol) onGetChain(s network.Stream) {
	// get request data
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		log.Error(err)

		_ = s.Reset()

		return
	}

	// unmarshal it
	var data = &pb.Message{}

	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Error(err)
		return
	}

	if !w.node.verifyMessage(s.Conn().RemotePeer(), data) {
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
}
