package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
	"log"
)

func (p *Protocol) Chain(s network.Stream, uuid string, getchain *ngtypes.GetChainPayload) bool {
	log.Printf("Sending Chain to %s. Message id: %s, Chain from vault@%d ...", s.Conn().RemotePeer(), uuid, getchain.VaultHeight)
	var blocks = make([]*ngtypes.Block, 0, ngtypes.BlockCheckRound)

	for i := getchain.VaultHeight * ngtypes.BlockCheckRound; i < (getchain.VaultHeight+1)*ngtypes.BlockCheckRound; i++ {
		b := p.node.Chain.GetBlockByHeight(i)
		if b == nil {
			log.Printf("Error: missing block@%d", i)
			return false
		}
		blocks = append(blocks, b)
	}

	payload, err := proto.Marshal(&ngtypes.ChainPayload{
		Vault:        p.node.Chain.GetVaultByHeight(getchain.VaultHeight),
		Blocks:       blocks,
		LatestHeight: p.node.Chain.GetLatestBlockHeight(),
	})
	if err != nil {
		log.Println("failed to sign pb data")
		return false
	}

	// create message data
	req := &ngtypes.P2PMessage{
		Header:  p.node.NewP2PHeader(uuid, false),
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

	ok := p.node.sendProtoMessage(s.Conn().RemotePeer(), chainMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	p.requests[req.Header.Uuid] = req
	log.Printf("Chain to: %s was sent. Message Id: %s", s.Conn().RemotePeer(), req.Header.Uuid)
	return true
}

func (p *Protocol) onChain(s network.Stream) {
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

	valid := p.node.authenticateMessage(&data, data.Header)

	if !valid {
		log.Println("Failed to authenticate message")
		return
	}

	var chain ngtypes.ChainPayload
	err = proto.Unmarshal(data.Payload, &chain)
	if err != nil {
		log.Println(err)
		return
	}

	err = p.node.Chain.PutVault(chain.Vault)
	if err != nil {
		log.Println(err)
		return
	}
	for i := 0; i < len(chain.Blocks); i++ {
		err := p.node.Chain.PutBlock(chain.Blocks[i])
		if err != nil {
			log.Println(err)
			return
		}
	}

	p.node.RemoteHeights.Store(s.Conn().RemotePeer(), chain.LatestHeight)

	log.Printf("Received Chain from %s. Message id:%s. From: %d To: %d LatestHeight: %d.", s.Conn().RemotePeer(), data.Header.Uuid, chain.Blocks[0].GetHeight(), chain.Blocks[len(chain.Blocks)-1].GetHeight(), chain.LatestHeight)

	if p.node.Chain.GetLatestBlockHeight()+ngtypes.BlockCheckRound < chain.LatestHeight {
		p.GetChain(s.Conn().RemotePeer(), chain.LatestHeight)
	} else {
		// locate request data and remove it if found
		_, ok := p.requests[data.Header.Uuid]
		if ok {
			// remove request from map as we have processed it here
			delete(p.requests, data.Header.Uuid)
		} else {
			log.Println("Failed to locate request data object for response")
			//return`
		}

		p.doneCh <- true
	}
}
