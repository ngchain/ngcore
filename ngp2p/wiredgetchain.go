package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/ngin-network/ngcore/ngp2p/pb"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
)

func (w *Wired) GetChain(remotePeerId peer.ID) bool {
	localHeight := w.node.Chain.GetLatestBlockHeight()
	vaultHeight := (localHeight + 1) / ngtypes.BlockCheckRound
	payload, err := proto.Marshal(&pb.GetChainPayload{
		VaultHeight: vaultHeight,
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
	signature, err := w.node.signProtoMessage(req)
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
	w.requests[req.Header.Uuid] = req
	log.Infof("getchain to: %s was sent. Message Id: %s, request vault height: %d", remotePeerId, req.Header.Uuid, vaultHeight)
	return true
}

func (w *Wired) onGetChain(s network.Stream) {
	// get request data
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Error(err)
		return
	}
	s.Close()

	// unmarshal it
	var data pb.Message
	err = proto.Unmarshal(buf, &data)
	if err != nil {
		log.Error(err)
		return
	}

	var getchain pb.GetChainPayload
	err = proto.Unmarshal(data.Payload, &getchain)
	if err != nil {
		log.Error(err)
		return
	}

	if w.node.authenticateMessage(&data, data.Header) {
		log.Infof("Received getchain request from %s. From Vault@%d", s.Conn().RemotePeer(), getchain.VaultHeight)

		// Chain
		localHeight := w.node.Chain.GetLatestBlockHeight()
		if localHeight < getchain.VaultHeight {
			w.Reject(s, data.Header.Uuid)
			return
		}

		w.Chain(s, data.Header.Uuid, &getchain)
		return
	}
}
