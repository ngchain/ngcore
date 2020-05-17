package ngp2p

import (
	"io/ioutil"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

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

func (w *wiredProtocol) onGetChain(s network.Stream) {
	// get request data
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		log.Error(err)

		_ = s.Reset()

		return
	}

	// unmarshal it
	var data = &Message{}

	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Error(err)
		return
	}

	if !verifyMessage(s.Conn().RemotePeer(), data) {
		log.Errorf("Failed to authenticate message")
		return
	}

	var payload = &GetChainPayload{}

	err = proto.Unmarshal(data.Payload, payload)
	if err != nil {
		log.Error(err)
		return
	}

	remoteID := s.Conn().RemotePeer()
	_ = s.Close()

	log.Debugf("Received getchain request from %s. Requested %d to %d", remoteID, payload.From, payload.To)
}
