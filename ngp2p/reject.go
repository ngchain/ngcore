package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
	"log"
)

func (p *Protocol) Reject(s network.Stream, uuid string) {
	log.Println("Failed to authenticate message")
	log.Printf("%s: Sending Reject to %s. Message id: %s...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), uuid)
	resp := &ngtypes.P2PMessage{
		Header:  p.node.NewP2PHeader(uuid, false),
		Payload: nil,
	}

	// sign the data
	signature, err := p.node.signProtoMessage(resp)
	if err != nil {
		log.Println("failed to sign response")
		return
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	if ok := p.node.sendProtoMessage(s.Conn().RemotePeer(), rejectMethod, resp); ok {
		log.Printf("%s: Reject to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}
}

// remote reject handler
func (p *Protocol) onReject(s network.Stream) {
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

	// locate request data and remove it if found
	_, ok := p.requests[data.Header.Uuid]
	if ok {
		// remove request from map as we have processed it here
		delete(p.requests, data.Header.Uuid)
	} else {
		log.Println("Failed to locate request data object for response")
		//return
	}

	log.Printf("%s: Received Reject from %s. Message id:%s. Message: %s.", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.Header.Uuid, data.Payload)
	p.doneCh <- true
	p.node.Network().ClosePeer(s.Conn().RemotePeer())
}
