package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
	"log"
)

func (p *Protocol) Blocks(s network.Stream, uuid string, getblocks *ngtypes.GetBlocksPayload) bool {
	log.Printf("%s: Sending Blocks to %s. Message id: %s, Blocks from %d to %d ...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), uuid, getblocks.FromCheckpoint, getblocks.ToCheckpoint)
	var blocks = make([]*ngtypes.Block, getblocks.ToCheckpoint-getblocks.FromCheckpoint)
	for i := getblocks.FromCheckpoint + 1; i <= getblocks.ToCheckpoint; i++ {
		b := p.node.Chain.GetBlockByHeight(i)
		if b == nil {
			log.Println("Error: missing block @ height:", i)
			return false
		}
		blocks[i-getblocks.FromCheckpoint-1] = b
	}

	payload, err := proto.Marshal(&ngtypes.BlocksPayload{
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

	ok := p.node.sendProtoMessage(s.Conn().RemotePeer(), blocksMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	p.requests[req.Header.Uuid] = req
	log.Printf("%s: Blocks to: %s was sent. Message Id: %s", p.node.ID(), s.Conn().RemotePeer(), req.Header.Uuid)
	return true
}

func (p *Protocol) onBlocks(s network.Stream) {
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

	var blocks ngtypes.BlocksPayload
	err = proto.Unmarshal(data.Payload, &blocks)
	if err != nil {
		log.Println(err)
		return
	}

	for i := 0; i < len(blocks.Blocks); i++ {
		err := p.node.Chain.PutBlock(blocks.Blocks[i])
		if err != nil {
			log.Println(err)
			return
		}
	}

	// locate request data and remove it if found
	_, ok := p.requests[data.Header.Uuid]
	if ok {
		// remove request from map as we have processed it here
		delete(p.requests, data.Header.Uuid)
	} else {
		log.Println("Failed to locate request data object for response")
		//return`
	}

	log.Printf("%s: Received Blocks from %s. Message id:%s. From: %d To: %d LatestHeight: %d.", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.Header.Uuid, blocks.Blocks[0].GetHeight(), blocks.Blocks[len(blocks.Blocks)-1].GetHeight(), blocks.LatestHeight)
	p.doneCh <- true
}
