package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/ngchain/ngcore/ngp2p/pb"
	"io/ioutil"
)

func (w *Wired) GetChain(remotePeerId peer.ID, requestHeight uint64) bool {
	payload, err := proto.Marshal(&pb.GetChainPayload{
		VaultHeight: requestHeight,
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

	ok := w.node.sendProtoMessage(remotePeerId, getChainMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	w.requests.Store(req.Header.Uuid, req)
	log.Infof("getchain to: %s was sent. Message Id: %s, request vault height: %d", remotePeerId, req.Header.Uuid, requestHeight)
	return true
}

func (w *Wired) onGetChain(s network.Stream) {
	// get request data
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		_ = s.Reset()
		log.Error(err)
		return
	}
	_ = s.Close()

	// unmarshal it
	var data = &pb.Message{}
	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Error(err)
		return
	}

	if !w.node.authenticateMessage(data) {
		log.Errorf("Failed to authenticate message")
		return
	}

	var getchain = &pb.GetChainPayload{}
	err = proto.Unmarshal(data.Payload, getchain)
	if err != nil {
		log.Error(err)
		return
	}

	log.Infof("Received getchain request from %s. Requested vault@%d", s.Conn().RemotePeer(), getchain.VaultHeight)

	// Chain
	localHeight := w.node.Chain.GetLatestBlockHeight()
	if localHeight < getchain.VaultHeight {
		go w.Reject(s, data.Header.Uuid)
		return
	}

	go w.Chain(s, data.Header.Uuid, getchain)
	return
}
