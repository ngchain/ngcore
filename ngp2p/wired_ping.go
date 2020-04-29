package ngp2p

import (
	"fmt"
	"io/ioutil"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/utils"
)

func (w *wiredProtocol) ping(remotePeerID peer.ID, from [][]byte) bool {
	payload, err := utils.Proto.Marshal(&pb.PingPayload{
		From:           0,
		Latest:         0,
		CheckpointHash: nil,
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
func (w *wiredProtocol) onPing(s network.Stream) (*pb.PingPayload, error) {
	// get request data
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		return nil, err
	}

	_ = s.Close()

	// unmarshal it
	var data = &pb.Message{}

	err = proto.Unmarshal(buf, data)
	if err != nil {
		return nil, err
	}

	if !w.node.verifyMessage(s.Conn().RemotePeer(), data) {
		return nil, fmt.Errorf("failed to authenticate message")
	}

	ping := &pb.PingPayload{}

	err = proto.Unmarshal(data.Payload, ping)
	if err != nil {
		return nil, err
	}

	log.Debugf("Received ping request from %s. Remote height: %d", s.Conn().RemotePeer(), ping.Latest)

	return ping, nil
}
