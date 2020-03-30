package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
	"io/ioutil"
)

func (w *Wired) Ping(remotePeerId peer.ID) bool {
	payload, err := proto.Marshal(&pb.PingPongPayload{
		BlockHeight:     w.node.Chain.GetLatestBlockHeight(),
		VaultHeight:     w.node.Chain.GetLatestVaultHeight(),
		LatestBlockHash: w.node.Chain.GetLatestBlockHash(),
		LatestVaultHash: w.node.Chain.GetLatestVaultHash(),
	})
	if err != nil {
		log.Errorf("failed to sign pb data")
		return false
	}

	// create message data
	req := &pb.Message{
		Header:  w.node.NewHeader(uuid.New().String()),
		Payload: payload,
	}

	// sign the data
	signature, err := w.node.signMessage(req)
	if err != nil {
		log.Errorf("failed to sign pb data")
		return false
	}

	// add the signature to the message
	req.Header.Sign = signature

	ok := w.node.sendProtoMessage(remotePeerId, pingMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	w.requests.Store(req.Header.Uuid, req)
	log.Infof("Sent Ping to: %s was sent. Message Id: %s.", remotePeerId, req.Header.Uuid)
	return true
}

// remote peer requests handler
func (w *Wired) onPing(s network.Stream) {
	// get request data
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Error(err)
		return
	}
	s.Close()

	// unmarshal it
	var data = &pb.Message{}
	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Error(err)
		go w.Reject(s, data.Header.Uuid)
		return
	}

	if !w.node.authenticateMessage(data) {
		log.Errorf("Failed to authenticate message")
		return
	}

	var ping = &pb.PingPongPayload{}
	err = proto.Unmarshal(data.Payload, ping)
	if err != nil {
		log.Error(err)
		go w.Reject(s, data.Header.Uuid)
		return
	}

	log.Infof("Received ping request from %s. Remote height: %d", s.Conn().RemotePeer(), ping.BlockHeight)

	// Pong
	w.node.Peerstore().AddAddrs(s.Conn().RemotePeer(), []core.Multiaddr{s.Conn().RemoteMultiaddr()}, ngtypes.TargetTime*ngtypes.BlockCheckRound*ngtypes.BlockCheckRound)
	go w.Pong(s, data.Header.Uuid)

	return
}
