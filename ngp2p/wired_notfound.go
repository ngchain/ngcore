package ngp2p

import (
	"io/ioutil"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/pb"
)

// notFound will reply notFound message to remote node.
func (w *wiredProtocol) notFound(peerID peer.ID, uuid string, blockHash []byte) {
	log.Debugf("Sending notfound to %s. Message id: %s...", peerID, uuid)

	resp := &pb.Message{
		Header:  w.node.NewHeader(uuid),
		Payload: blockHash,
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
	if ok := w.node.sendProtoMessage(peerID, notFoundMethod, resp); ok {
		log.Debugf("notfound to %s sent.", peerID)
	}
}

// onNotFound is a remote notfound handler. When received a notfound, local node is running on a wrong chain
func (w *wiredProtocol) onNotFound(s network.Stream) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		log.Error(err)

		_ = s.Reset()

		return
	}

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

	remoteID := s.Conn().RemotePeer()
	_ = s.Close()

	log.Debugf("Received notfound from %s. Message id:%s. Message: %s.", remoteID, data.Header.Uuid, data.Payload)
}
