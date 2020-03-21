package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngchain/ngcore/ngp2p/pb"
	"io/ioutil"
)

func (w *Wired) Reject(s network.Stream, uuid string) {
	log.Warning("Failed to authenticate message")
	log.Infof("Sending Reject to %s. Message id: %s...", s.Conn().RemotePeer(), uuid)
	resp := &pb.Message{
		Header:  w.node.NewHeader(uuid),
		Payload: nil,
	}

	// sign the data
	signature, err := w.node.signProtoMessage(resp)
	if err != nil {
		log.Errorf("failed to sign response")
		return
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	if ok := w.node.sendProtoMessage(s.Conn().RemotePeer(), rejectMethod, resp); ok {
		log.Infof("Reject to %s sent.", s.Conn().RemotePeer().String())
	}
}

// remote reject handler
func (w *Wired) onReject(s network.Stream) {
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

	// locate request data and remove it if found
	_, ok := w.requests[data.Header.Uuid]
	if ok {
		// remove request from map as we have processed it here
		delete(w.requests, data.Header.Uuid)
	} else {
		log.Error("Failed to locate request data object for response")
		//return
	}

	log.Infof("Received Reject from %s. Message id:%s. Message: %s.", s.Conn().RemotePeer(), data.Header.Uuid, data.Payload)
	w.node.Network().ClosePeer(s.Conn().RemotePeer())
}
