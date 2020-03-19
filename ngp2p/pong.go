package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngin-network/ngcore/ngp2p/pb"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
)

func (p *Protocol) Pong(s network.Stream, uuid string) bool {
	log.Infof("%s: Sending Pong to %s. Message id: %s...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), uuid)

	payload, err := proto.Marshal(&pb.PingPongPayload{
		BlockHeight: p.node.Chain.GetLatestBlockHeight(),
	})
	if err != nil {
		log.Error("failed to sign pb data")
		return false
	}

	resp := &pb.P2PMessage{
		Header:  p.node.NewP2PHeader(uuid, false),
		Payload: payload,
	}

	// sign the data
	signature, err := p.node.signProtoMessage(resp)
	if err != nil {
		log.Error("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	if ok := p.node.sendProtoMessage(s.Conn().RemotePeer(), pongMethod, resp); ok {
		log.Infof("%s: Pong to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}
	return true
}

// remote ping response handler
func (p *Protocol) onPong(s network.Stream) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Error(err)
		return
	}
	s.Close()

	// unmarshal it
	var data pb.P2PMessage
	err = proto.Unmarshal(buf, &data)
	if err != nil {
		log.Error(err)
		return
	}

	valid := p.node.authenticateMessage(&data, data.Header)

	if !valid {
		log.Error("Failed to authenticate message")
		return
	}

	var pong pb.PingPongPayload
	err = proto.Unmarshal(data.Payload, &pong)
	if err != nil {
		log.Error(err)
		return
	}

	log.Infof("Received Pong from %s. Message id:%s. Message: %d.", s.Conn().RemotePeer(), data.Header.Uuid, pong.BlockHeight)

	p.node.RemoteHeights.Store(s.Conn().RemotePeer().String(), pong.BlockHeight)

	if p.node.Chain.GetLatestBlockHeight()+ngtypes.BlockCheckRound < pong.BlockHeight {
		log.Infof("start syncing with %s", s.Conn().RemotePeer())
		go p.GetChain(s.Conn().RemotePeer())
	} else {
		log.Infof("synced with %s", s.Conn().RemotePeer())
		// locate request data and remove it if found
		_, ok := p.requests[data.Header.Uuid]
		if ok {
			// remove request from map as we have processed it here
			delete(p.requests, data.Header.Uuid)
		} else {
			log.Errorf("Failed to locate request data object for response")
			//return
		}
	}

	p.node.Peerstore().AddAddrs(s.Conn().RemotePeer(), []core.Multiaddr{s.Conn().RemoteMultiaddr()}, ngtypes.TargetTime*ngtypes.BlockCheckRound*ngtypes.BlockCheckRound)
	p.doneCh <- true
}
