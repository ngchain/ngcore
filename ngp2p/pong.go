package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
	"log"
)

func (p *Protocol) Pong(s network.Stream, uuid string) bool {
	log.Printf("%s: Sending Pong to %s. Message id: %s...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), uuid)

	payload, err := proto.Marshal(&ngtypes.PingPongPayload{
		BlockHeight: p.node.blockChain.GetLatestBlockHeight(),
	})
	if err != nil {
		log.Println("failed to sign pb data")
		return false
	}

	resp := &ngtypes.P2PMessage{
		Header:  p.node.NewP2PHeader(uuid, false),
		Payload: payload,
	}

	// sign the data
	signature, err := p.node.signProtoMessage(resp)
	if err != nil {
		log.Println("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	if ok := p.node.sendProtoMessage(s.Conn().RemotePeer(), pongMethod, resp); ok {
		log.Printf("%s: Pong to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}
	return true
}

// remote ping response handler
func (p *Protocol) onPong(s network.Stream) {
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

	var pong ngtypes.PingPongPayload
	err = proto.Unmarshal(data.Payload, &pong)
	if err != nil {
		log.Println(err)
		return
	}

	if p.node.blockChain.GetLatestBlockHeight()+ngtypes.CheckRound < pong.BlockHeight {
		log.Println("start syncManager to sync with", s.Conn().RemotePeer())
		go p.GetBlocks(s.Conn().RemotePeer(), pong.BlockHeight)
	}

	// locate request data and remove it if found
	_, ok := p.requests[data.Header.Uuid]
	if ok {
		// remove request from map as we have processed it here
		delete(p.requests, data.Header.Uuid)
	} else {
		log.Println("Failed to locate request data object for response")
		return
	}

	p.node.Peerstore().AddAddrs(s.Conn().RemotePeer(), []core.Multiaddr{s.Conn().RemoteMultiaddr()}, ngtypes.TargetTime * ngtypes.CheckRound * ngtypes.CheckRound)
	log.Printf("%s: Received Pong from %s. Message id:%s. Message: %d.", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.Header.Uuid, pong.BlockHeight)
	p.doneCh <- true
}
