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

func (p *Protocol) GetBlocks(remotePeerId peer.ID, remoteHeight uint64) bool {
	localHeight := p.node.blockChain.GetLatestBlockHeight()

	payload, err := proto.Marshal(&ngtypes.GetBlocksPayload{
		FromCheckpoint: localHeight - (localHeight % ngtypes.CheckRound),
		ToCheckpoint:       localHeight - (localHeight % ngtypes.CheckRound),
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

	ok := p.node.sendProtoMessage(remotePeerId, getblocksMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	p.requests[req.Header.Uuid] = req
	log.Printf("%s: getBlocks to: %s was sent. Message Id: %s, Message: %s", p.node.ID(), remotePeerId, req.Header.Uuid, req.Payload)
	return true
}

func (p *Protocol) onGetBlocks(s network.Stream) {
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

	var getblocks ngtypes.GetBlocksPayload
	err = proto.Unmarshal(data.Payload, &getblocks)
	if err != nil {
		log.Println(err)
		return
	}

	if p.node.authenticateMessage(&data, data.Header) {

		// Blocks
		if getblocks.FromCheckpoint%ngtypes.CheckRound != 0 {
			p.Reject(s, data.Header.Uuid)
		}

		if getblocks.ToCheckpoint < getblocks.FromCheckpoint {
			p.Reject(s, data.Header.Uuid)
		}

		localHeight := p.node.blockChain.GetLatestBlockHeight()
		if localHeight < getblocks.ToCheckpoint {
			p.Reject(s, data.Header.Uuid)
		}

		p.Blocks(s.Conn().RemotePeer(), &getblocks)
		return
	}
}
