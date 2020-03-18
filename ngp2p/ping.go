package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
)

func (p *Protocol) Ping(remotePeerId peer.ID) bool {
	payload, err := proto.Marshal(&ngtypes.PingPongPayload{
		BlockHeight: p.node.Chain.GetLatestBlockHeight(),
	})
	if err != nil {
		log.Errorf("failed to sign pb data")
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
		log.Errorf("failed to sign pb data")
		return false
	}

	// add the signature to the message
	req.Header.Sign = signature

	ok := p.node.sendProtoMessage(remotePeerId, pingMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	p.requests[req.Header.Uuid] = req
	log.Infof("Sent Ping to: %s was sent. Message Id: %s.", remotePeerId, req.Header.Uuid)
	return true
}

// remote peer requests handler
func (p *Protocol) onPing(s network.Stream) {
	// get request data
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

	var ping ngtypes.PingPongPayload
	err = proto.Unmarshal(data.Payload, &ping)
	if err != nil {
		log.Error(err)
		return
	}

	log.Infof("Received ping request from %s. Remote height: %d", s.Conn().RemotePeer(), ping.BlockHeight)
	if p.node.authenticateMessage(&data, data.Header) {
		// Pong
		p.node.Peerstore().AddAddrs(s.Conn().RemotePeer(), []core.Multiaddr{s.Conn().RemoteMultiaddr()}, ngtypes.TargetTime*ngtypes.BlockCheckRound*ngtypes.BlockCheckRound)
		go p.Pong(s, data.Header.Uuid)
		return
	} else {
		// Reject
		go p.Reject(s, data.Header.Uuid)
		return
	}

}
