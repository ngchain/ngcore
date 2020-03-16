package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
	"log"
)

func (p *Protocol) GetChain(remotePeerId peer.ID) bool {
	localHeight := p.node.Chain.GetLatestBlockHeight()
	vaultHeight := localHeight - (localHeight % ngtypes.BlockCheckRound)
	payload, err := proto.Marshal(&ngtypes.GetChainPayload{
		VaultHeight: vaultHeight,
	})
	if err != nil {
		log.Println("failed to sign pb data")
		return false
	}

	// create message data
	req := &ngtypes.P2PMessage{
		Header:  p.node.NewP2PHeader(uuid.New().String(), false),
		Payload: payload,
	}

	// sign the data
	signature, err := p.node.signProtoMessage(req)
	if err != nil {
		log.Println("failed to sign pb data")
		return false
	}

	// add the signature to the message
	req.Header.Sign = signature

	ok := p.node.sendProtoMessage(remotePeerId, getChainMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	p.requests[req.Header.Uuid] = req
	log.Printf("%s: getchain to: %s was sent. Message Id: %s, request vault height: %d", p.node.ID(), remotePeerId, req.Header.Uuid, vaultHeight)
	return true
}

func (p *Protocol) onGetChain(s network.Stream) {
	// get request data
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Println(err)
		return
	}
	s.Close()

	// unmarshal it
	var data ngtypes.P2PMessage
	err = proto.Unmarshal(buf, &data)
	if err != nil {
		log.Println(err)
		return
	}

	var getchain ngtypes.GetChainPayload
	err = proto.Unmarshal(data.Payload, &getchain)
	if err != nil {
		log.Println(err)
		return
	}

	if p.node.authenticateMessage(&data, data.Header) {
		log.Printf("Received getchain request from %s. From Vault@%d", s.Conn().RemotePeer(), getchain.VaultHeight)

		// Chain
		localHeight := p.node.Chain.GetLatestBlockHeight()
		if localHeight < getchain.VaultHeight {
			p.Reject(s, data.Header.Uuid)
			return
		}

		p.Chain(s, data.Header.Uuid, &getchain)
		return
	}
}
