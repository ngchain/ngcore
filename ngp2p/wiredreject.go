package ngp2p

import (
	"io/ioutil"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngp2p/pb"
)

// Reject will reply Reject message to remote node
func (w *wired) Reject(peerID peer.ID, uuid string) {
	log.Debugf("Sending Reject to %s. Message id: %s...", peerID, uuid)
	resp := &pb.Message{
		Header:  w.node.NewHeader(uuid),
		Payload: nil,
	}

	// sign the data
	signature, err := w.node.signMessage(resp)
	if err != nil {
		log.Errorf("failed to sign response")
		return
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	if ok := w.node.sendProtoMessage(peerID, rejectMethod, resp); ok {
		log.Debugf("Reject to %s sent.", peerID)
	}
}

// remote reject handler
func (w *wired) onReject(s network.Stream) {
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
		return
	}

	if !w.node.verifyResponse(data) {
		log.Errorf("Failed to verify response")
		return
	}

	if !w.node.authenticateMessage(s.Conn().RemotePeer(), data) {
		log.Errorf("Failed to authenticate message")
		return
	}

	log.Debugf("Received Reject from %s. Message id:%s. Message: %s.", remotePeerID, data.Header.Uuid, data.Payload)
}
