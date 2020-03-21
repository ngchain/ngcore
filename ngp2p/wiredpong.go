package ngp2p

import (
	"bytes"
	"github.com/gogo/protobuf/proto"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngin-network/ngcore/ngp2p/pb"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
)

func (w *Wired) Pong(s network.Stream, uuid string) bool {
	log.Infof("Sending Pong to %s. Message id: %s...", s.Conn().RemotePeer(), uuid)

	payload, err := proto.Marshal(&pb.PingPongPayload{
		BlockHeight:     w.node.Chain.GetLatestBlockHeight(),
		VaultHeight:     w.node.Chain.GetLatestVaultHeight(),
		LatestBlockHash: w.node.Chain.GetLatestBlockHash(),
		LatestVaultHash: w.node.Chain.GetLatestVaultHash(),
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
	signature, err := w.node.signProtoMessage(resp)
	if err != nil {
		log.Error("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	if ok := w.node.sendProtoMessage(s.Conn().RemotePeer(), pongMethod, resp); ok {
		log.Infof("Pong to %s sent.", s.Conn().RemotePeer().String())
	}
	return true
}

// remote ping response handler
func (w *Wired) onPong(s network.Stream) {
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Error(err)
		return
	}
	s.Close()

	// unmarshal it
	var data = &pb.Message{}
	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Error(err)
		return
	}

	if !w.node.verifyResponse(data.Header) {
		log.Error("Failed to authenticate message")
		return
	}

	var pong = &pb.PingPongPayload{}
	err = proto.Unmarshal(data.Payload, pong)
	if err != nil {
		log.Error(err)
		return
	}

	log.Infof("Received Pong from %s. Message id:%s. Remote height: %d.", s.Conn().RemotePeer(), data.Header.Uuid, pong.BlockHeight)
	w.node.Peerstore().AddAddrs(s.Conn().RemotePeer(), []core.Multiaddr{s.Conn().RemoteMultiaddr()}, ngtypes.TargetTime*ngtypes.BlockCheckRound*ngtypes.BlockCheckRound)

	w.node.RemoteHeights.Store(s.Conn().RemotePeer().String(), pong.BlockHeight)

	localVaultHeight := w.node.Chain.GetLatestVaultHeight()
	localVaultHash := w.node.Chain.GetLatestVaultHash()
	localBlockHeight := w.node.Chain.GetLatestBlockHeight()

	if localVaultHeight < pong.VaultHeight {
		// start sync
		log.Infof("start syncing with %s", s.Conn().RemotePeer())
		if w.node.isStrictMode {
			requestHeight := (localBlockHeight + 1) / ngtypes.BlockCheckRound
			go w.GetChain(s.Conn().RemotePeer(), requestHeight)
		} else {
			go w.GetChain(s.Conn().RemotePeer(), pong.VaultHeight-2)
		}
		return
	}

	if localVaultHeight == pong.VaultHeight && bytes.Compare(localVaultHash, pong.LatestVaultHash) != 0 {
		// start fork
		log.Infof("start switching to the chain of %s", s.Conn().RemotePeer())
		go w.GetChain(s.Conn().RemotePeer(), pong.VaultHeight)
		return
	}
}
