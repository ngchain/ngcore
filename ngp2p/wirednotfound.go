package ngp2p

import (
	"io/ioutil"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"

	"github.com/ngchain/ngcore/ngp2p/pb"
)

// NotFound will reply NotFound message to remote node
func (w *Wired) NotFound(s network.Stream, uuid string) {
	log.Warning("Failed to authenticate message")
	log.Infof("Sending notfound to %s. Message id: %s...", s.Conn().RemotePeer(), uuid)
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
	if ok := w.node.sendProtoMessage(s.Conn().RemotePeer(), notfoundMethod, resp); ok {
		log.Infof("notfound to %s sent.", s.Conn().RemotePeer().String())
	}
}

// onNotFound is a remote notfound handler
func (w *Wired) onNotFound(s network.Stream) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		_ = s.Reset()
		log.Error(err)
		return
	}
	_ = s.Close()

	// unmarshal it
	var data pb.Message
	err = proto.Unmarshal(buf, &data)
	if err != nil {
		log.Error(err)
		return
	}

	// locate request data and remove it if found
	_, ok := w.requests.Load(data.Header.Uuid)
	if ok {
		// remove request from map as we have processed it here
		w.requests.Delete(data.Header.Uuid)
	} else {
		log.Error("Failed to locate request data object for response")
	}

	log.Infof("Received notfound from %s. Message id:%s. Message: %s.", s.Conn().RemotePeer(), data.Header.Uuid, data.Payload)
	// _ = w.node.Network().ClosePeer(s.Conn().RemotePeer())
}
