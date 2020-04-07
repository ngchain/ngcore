package ngp2p

import (
	"bytes"
	"io/ioutil"

	"github.com/gogo/protobuf/proto"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
)

func (w *Wired) Pong(s network.Stream, uuid string) bool {
	log.Debugf("Sending Pong to %s. Message id: %s...", s.Conn().RemotePeer(), uuid)

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
	if ok := w.node.sendProtoMessage(s.Conn().RemotePeer(), pongMethod, resp); ok {
		log.Debugf("Pong to %s sent.", s.Conn().RemotePeer().String())
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

	remoteID := s.Conn().RemotePeer()
	_ = s.Close()

	log.Debugf("Received Pong from %s. Message id:%s. Remote height: %d.", remoteID, data.Header.Uuid, pong.BlockHeight)
	w.node.Peerstore().AddAddrs(remoteID, []core.Multiaddr{s.Conn().RemoteMultiaddr()}, ngtypes.TargetTime*ngtypes.BlockCheckRound*ngtypes.BlockCheckRound)

	w.node.RemoteHeights.Store(remoteID.String(), pong.BlockHeight)

	localVaultHeight := w.node.consensus.GetLatestVaultHeight()
	localVaultHash := w.node.consensus.GetLatestVaultHash()
	localBlockHeight := w.node.consensus.GetLatestBlockHeight()

	if !w.node.isStrictMode && !w.node.isInitialized.Load() && w.node.consensus.GetLatestBlockHeight() == 0 {
		go w.GetChain(remoteID, pong.VaultHeight-2)
		return
	}

	if localVaultHeight < pong.VaultHeight {
		// start sync
		log.Infof("start syncing with %s", remoteID)
		if w.node.isStrictMode {
			requestHeight := (localBlockHeight + 1) / ngtypes.BlockCheckRound
			go w.GetChain(remoteID, requestHeight)
		} else {
			go w.GetChain(remoteID, pong.VaultHeight-2)
		}
		return
	}

	if localVaultHeight == pong.VaultHeight && !bytes.Equal(localVaultHash, pong.LatestVaultHash) {
		// start fork
		log.Infof("start switching to the chain of %s", remoteID)
		go w.GetChain(remoteID, pong.VaultHeight)
		return
	}
}
