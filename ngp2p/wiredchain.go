package ngp2p

import (
	"io/ioutil"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// chain will send peer the specific vault's chain, which's len is not must be full BlockCheckRound num
func (w *wired) chain(peerID peer.ID, uuid string, blocks ...*ngtypes.Block) bool {
	if len(blocks) == 0 {
		return false
	}
	log.Debugf("Sending chain to %s. Message id: %s, chain from block@%d ...",
		peerID, uuid, blocks[0].GetHeight())

	payload, err := utils.Proto.Marshal(&pb.ChainPayload{
		Blocks:       blocks,
		LatestHeight: w.node.consensus.GetLatestBlockHeight(),
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

func (w *wired) onChain(s network.Stream) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		_ = s.Reset()
		log.Error(err)

		return
	}

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

	var payload = &pb.ChainPayload{}
	err = proto.Unmarshal(data.Payload, payload)
	if err != nil {
		log.Error(err)
		return
	}

	remotePeerID := s.Conn().RemotePeer()
	_ = s.Close()

	if len(payload.Blocks) == 0 {
		return
	}

	log.Debugf("Received chain from %s. Message id:%s. From: %d To: %d LatestHeight: %d.",
		remotePeerID, data.Header.Uuid,
		payload.Blocks[0].GetHeight(),
		payload.Blocks[len(payload.Blocks)-1].GetHeight(),
		payload.LatestHeight,
	)

	w.node.RemoteHeights.Store(remotePeerID, payload.LatestHeight)

	if w.forkManager.enabled.Load() {
		w.node.forkManager.handleChain(remotePeerID, payload)
	}
}
