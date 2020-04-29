package ngp2p

import (
	"io/ioutil"

	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
)

func (w *wiredProtocol) pong(peerID peer.ID, uuid string) bool {
	log.Debugf("Sending pong to %s. Message id: %s...", peerID, uuid)

	resp := &pb.Message{
		Header:  w.node.NewHeader(uuid),
		Payload: nil,
	}

	// sign the data
	signature, err := w.node.signMessage(resp)
	if err != nil {
		log.Error("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	if ok := w.node.sendProtoMessage(peerID, pongMethod, resp); ok {
		log.Debugf("pong to %s sent.", peerID.String())
	}

	return true
}

// remote Pong response handler.
func (w *wiredProtocol) onPong(s network.Stream) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		log.Error(err)

		_ = s.Reset()

		return
	}

	remotePeerID := s.Conn().RemotePeer()
	_ = s.Close()

	// unmarshal it
	var data = &pb.Message{}

	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Error(err)
		return
	}

	if !w.node.verifyResponse(data) {
		log.Errorf("Failed to verify response")
		return
	}

	if !w.node.verifyMessage(s.Conn().RemotePeer(), data) {
		log.Errorf("Failed to authenticate message")
		return
	}

	log.Debugf("Received pong from %s. Message id:%s.",
		remotePeerID,
		data.Header.Uuid,
	)

	w.node.Peerstore().AddAddrs(
		remotePeerID,
		[]core.Multiaddr{s.Conn().RemoteMultiaddr()},
		ngtypes.TargetTime*ngtypes.BlockCheckRound, // add live time
	)
}
