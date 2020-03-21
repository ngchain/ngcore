package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngin-network/ngcore/ngp2p/pb"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
)

func (w *Wired) Pong(s network.Stream, uuid string) bool {
	log.Infof("%s: Sending Pong to %s. Message id: %s...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), uuid)

	payload, err := proto.Marshal(&pb.PingPongPayload{
		BlockHeight: w.node.Chain.GetLatestBlockHeight(),
	})
	if err != nil {
		log.Error("failed to sign pb data")
		return false
	}

	resp := &pb.Message{
		Header:  w.node.NewHeader(uuid),
		Payload: payload,
	}

	// sign the data
	signature, err := w.node.signProtoMessage(resp)
	if err != nil {
		log.Error("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	if ok := w.node.sendProtoMessage(s.Conn().RemotePeer(), pongMethod, resp); ok {
		log.Infof("%s: Pong to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}
	return true
}

// remote ping response handler
func (w *Wired) onPong(s network.Stream) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Error(err)
		return
	}
	s.Close()

	// unmarshal it
	var data pb.Message
	err = proto.Unmarshal(buf, &data)
	if err != nil {
		log.Error(err)
		return
	}

	valid := w.node.authenticateMessage(&data, data.Header)

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

	log.Infof("Received Pong from %s. Message id:%s. Remote height: %d.", s.Conn().RemotePeer(), data.Header.Uuid, pong.BlockHeight)

	w.node.RemoteHeights.Store(s.Conn().RemotePeer().String(), pong.BlockHeight)

	if w.node.Chain.GetLatestBlockHeight()+ngtypes.BlockCheckRound < pong.BlockHeight {
		log.Infof("start syncing with %s", s.Conn().RemotePeer())
		go w.GetChain(s.Conn().RemotePeer())
	} else {
		log.Infof("synced with %s", s.Conn().RemotePeer())
		// locate request data and remove it if found
		_, ok := w.requests[data.Header.Uuid]
		if ok {
			// remove request from map as we have processed it here
			delete(w.requests, data.Header.Uuid)
		} else {
			log.Errorf("Failed to locate request data object for response")
			//return
		}
	}

	w.node.Peerstore().AddAddrs(s.Conn().RemotePeer(), []core.Multiaddr{s.Conn().RemoteMultiaddr()}, ngtypes.TargetTime*ngtypes.BlockCheckRound*ngtypes.BlockCheckRound)
}
