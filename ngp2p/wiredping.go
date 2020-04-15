package ngp2p

import (
	"io/ioutil"

	"github.com/google/uuid"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (w *wired) ping(remotePeerID peer.ID) bool {
	payload, err := utils.Proto.Marshal(&pb.PingPongPayload{
		BlockHeight:     w.node.consensus.GetLatestBlockHeight(),
		LatestBlockHash: w.node.consensus.GetLatestBlockHash(),
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

	ok := w.node.sendProtoMessage(remotePeerID, pingMethod, req)
	if !ok {
		log.Errorf("failed sending ping to: %s.", remotePeerID)
		return false
	}

	// store ref request so response handler has access to it
	w.requests.Store(req.Header.Uuid, req)
	log.Debugf("Sent ping to: %s was sent. Message Id: %s.", remotePeerID, req.Header.Uuid)
	return true
}

// remote peer requests handler
func (w *wired) onPing(s network.Stream) {
	// get request data
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		_ = s.Reset()
		log.Error(err)
		return
	}

	remotePeerID := s.Conn().RemotePeer()
	_ = s.Close()

	// unmarshal it
	var data = &pb.Message{}
	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Error(err)
		go w.reject(remotePeerID, data.Header.Uuid)

		return
	}

	if !w.node.authenticateMessage(s.Conn().RemotePeer(), data) {
		log.Errorf("Failed to authenticate message")
		return
	}

	var ping = &pb.PingPongPayload{}
	err = proto.Unmarshal(data.Payload, ping)
	if err != nil {
		log.Error(err)
		go w.reject(remotePeerID, data.Header.Uuid)

		return
	}

	log.Debugf("Received ping request from %s. Remote height: %d", s.Conn().RemotePeer(), ping.BlockHeight)

	// pong
	w.node.Peerstore().AddAddrs(
		s.Conn().RemotePeer(),
		[]core.Multiaddr{s.Conn().RemoteMultiaddr()},
		ngtypes.TargetTime*ngtypes.BlockCheckRound*ngtypes.BlockCheckRound,
	)

	go w.pong(remotePeerID, data.Header.Uuid)
}
