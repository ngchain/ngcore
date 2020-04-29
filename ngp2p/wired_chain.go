package ngp2p

import (
	"fmt"
	"io/ioutil"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// chain will send peer the specific vault's chain, which's len is not must be full BlockCheckRound num
func (w *wiredProtocol) chain(peerID peer.ID, uuid string, blocks ...*ngtypes.Block) bool {
	if len(blocks) == 0 {
		return false
	}

	log.Debugf("Sending chain to %s. Message id: %s, chain from block@%d ...",
		peerID, uuid, blocks[0].GetHeight())

	payload, err := utils.Proto.Marshal(&pb.ChainPayload{
		Blocks: blocks,
	})
	if err != nil {
		log.Errorf("failed to sign pb data")
		return false
	}

	// create message data
	req := &pb.Message{
		Header:  w.node.NewHeader(uuid),
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

	ok := w.node.sendProtoMessage(peerID, chainMethod, req)
	if !ok {
		log.Debugf("chain to: %s was sent. Message Id: %s", peerID, req.Header.Uuid)
		return false
	}

	// store ref request so response handler has access to it
	w.requests.Store(req.Header.Uuid, req)

	log.Debugf("chain to: %s was sent. Message Id: %s", peerID, req.Header.Uuid)

	return true
}

func (w *wiredProtocol) onChain(s network.Stream) (*pb.ChainPayload, error) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		return nil, err
	}

	// unmarshal it
	var data = &pb.Message{}
	err = proto.Unmarshal(buf, data)

	if err != nil {
		return nil, err
	}

	if !w.node.verifyResponse(data) {
		return nil, fmt.Errorf("failed to verify response")
	}

	if !w.node.verifyMessage(s.Conn().RemotePeer(), data) {
		return nil, fmt.Errorf("failed to verify message")
	}

	var payload = &pb.ChainPayload{}

	err = proto.Unmarshal(data.Payload, payload)
	if err != nil {
		return nil, err
	}

	remotePeerID := s.Conn().RemotePeer()
	_ = s.Close()

	if len(payload.Blocks) == 0 {
		return nil, fmt.Errorf("blocks filed is empty")
	}

	log.Debugf("Received chain from %s. Message id:%s. From: %d To: %d.",
		remotePeerID, data.Header.Uuid,
		payload.Blocks[0].GetHeight(),
		payload.Blocks[len(payload.Blocks)-1].GetHeight(),
	)

	return payload, nil
}
