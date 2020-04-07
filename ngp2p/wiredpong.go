package ngp2p

import (
	"bytes"
	"io/ioutil"

	"github.com/gogo/protobuf/proto"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
)

func (w *Wired) Pong(peerID peer.ID, uuid string) bool {
	log.Debugf("Sending Pong to %s. Message id: %s...", peerID, uuid)

	payload, err := proto.Marshal(&pb.PingPongPayload{
		BlockHeight:     w.node.consensus.GetLatestBlockHeight(),
		VaultHeight:     w.node.consensus.GetLatestVaultHeight(),
		LatestBlockHash: w.node.consensus.GetLatestBlockHash(),
		LatestVaultHash: w.node.consensus.GetLatestVaultHash(),
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
		log.Debugf("Pong to %s sent.", peerID.String())
	}
	return true
}

// remote ping response handler
func (w *Wired) onPong(s network.Stream) {
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

	log.Debugf("Received Pong from %s. Message id:%s. Remote height: %d.", remotePeerID, data.Header.Uuid, pong.BlockHeight)
	w.node.Peerstore().AddAddrs(remotePeerID, []core.Multiaddr{s.Conn().RemoteMultiaddr()}, ngtypes.TargetTime*ngtypes.BlockCheckRound*ngtypes.BlockCheckRound)

	w.node.RemoteHeights.Store(remotePeerID.String(), pong.BlockHeight)

	localVaultHeight := w.node.consensus.GetLatestVaultHeight()
	localVaultHash := w.node.consensus.GetLatestVaultHash()
	localBlockHeight := w.node.consensus.GetLatestBlockHeight()

	if !w.node.isStrictMode && !w.node.isInitialized.Load() && w.node.consensus.GetLatestBlockHeight() == 0 {
		go w.GetChain(remotePeerID, pong.VaultHeight-2)
		return
	}

	if localVaultHeight < pong.VaultHeight {
		// start sync
		log.Infof("start syncing with %s", remotePeerID)
		if w.node.isStrictMode {
			requestHeight := (localBlockHeight + 1) / ngtypes.BlockCheckRound
			go w.GetChain(remotePeerID, requestHeight)
		} else {
			go w.GetChain(remotePeerID, pong.VaultHeight-2)
		}
		return
	}

	if localVaultHeight == pong.VaultHeight && !bytes.Equal(localVaultHash, pong.LatestVaultHash) {
		// start fork
		log.Infof("start switching to the chain of %s", remotePeerID)
		go w.GetChain(remotePeerID, pong.VaultHeight)
		return
	}
}
