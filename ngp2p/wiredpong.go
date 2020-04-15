package ngp2p

import (
	"io/ioutil"

	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (w *wired) pong(peerID peer.ID, uuid string) bool {
	log.Debugf("Sending pong to %s. Message id: %s...", peerID, uuid)

	payload, err := utils.Proto.Marshal(&pb.PingPongPayload{
		BlockHeight:     w.node.consensus.GetLatestBlockHeight(),
		LatestBlockHash: w.node.consensus.GetLatestBlockHash(),
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

// remote ping response handler
func (w *wired) onPong(s network.Stream) {
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

	var pong = &pb.PingPongPayload{}
	err = proto.Unmarshal(data.Payload, pong)
	if err != nil {
		log.Error(err)
		return
	}

	log.Debugf("Received pong from %s. Message id:%s. Remote height: %d.",
		remotePeerID,
		data.Header.Uuid,
		pong.BlockHeight,
	)
	w.node.Peerstore().AddAddrs(
		remotePeerID,
		[]core.Multiaddr{s.Conn().RemoteMultiaddr()},
		ngtypes.TargetTime*ngtypes.BlockCheckRound*ngtypes.BlockCheckRound,
	)

	w.node.RemoteHeights.Store(remotePeerID.String(), pong.BlockHeight)
	localBlockHeight := w.node.consensus.GetLatestBlockHeight()

	// fast mode init
	if !w.node.isStrictMode && !w.node.isInitialized.Load() && w.node.consensus.GetLatestBlockHeight() == 0 {
		go w.getChain(remotePeerID, pong.BlockHeight-20, pong.BlockHeight)
		return
	}

	// start sync
	log.Infof("start syncing with %s", remotePeerID)
	if w.node.isStrictMode {
		go w.getChain(remotePeerID, localBlockHeight, pong.BlockHeight)
	} else {
		go w.getChain(remotePeerID, pong.BlockHeight-20, pong.BlockHeight)
	}
	return

	// if localVaultHeight == pong.VaultHeight && !bytes.Equal(localVaultHash, pong.LatestVaultHash) {
	// 	// start fork
	// 	log.Infof("start switching to the chain of %s", remotePeerID)
	// 	go w.GetChain(remotePeerID, pong.VaultHeight)
	// 	return
	// }
}
