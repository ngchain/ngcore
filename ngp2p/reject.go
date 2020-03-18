package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
)

func (p *Protocol) Reject(s network.Stream, uuid string) {
	log.Warning("Failed to authenticate message")
	log.Infof("Sending Reject to %s. Message id: %s...", s.Conn().RemotePeer(), uuid)
	resp := &ngtypes.P2PMessage{
		Header:  p.node.NewP2PHeader(uuid, false),
		Payload: nil,
	}

	// sign the data
	signature, err := p.node.signProtoMessage(resp)
	if err != nil {
		log.Errorf("failed to sign response")
		return
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	if ok := p.node.sendProtoMessage(s.Conn().RemotePeer(), rejectMethod, resp); ok {
		log.Infof("Reject to %s sent.", s.Conn().RemotePeer().String())
	}
}

// remote reject handler
func (p *Protocol) onReject(s network.Stream) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Error(err)
		return
	}
	s.Close()

	// unmarshal it
	var data ngtypes.P2PMessage
	err = proto.Unmarshal(buf, &data)
	if err != nil {
		log.Error(err)
		return
	}

	// locate request data and remove it if found
	_, ok := p.requests[data.Header.Uuid]
	if ok {
		// remove request from map as we have processed it here
		delete(p.requests, data.Header.Uuid)
	} else {
		log.Error("Failed to locate request data object for response")
		//return
	}

	log.Infof("Received Reject from %s. Message id:%s. Message: %s.", s.Conn().RemotePeer(), data.Header.Uuid, data.Payload)
	p.doneCh <- true
	p.node.Network().ClosePeer(s.Conn().RemotePeer())
}
