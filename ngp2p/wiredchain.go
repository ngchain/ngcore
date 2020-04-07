package ngp2p

import (
	"io/ioutil"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"

	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
)

// Chain will send peer the specific vault's chain, which's len is not must be full BlockCheckRound num
func (w *Wired) Chain(s network.Stream, uuid string, vault *ngtypes.Vault, blocks ...*ngtypes.Block) bool {
	log.Debugf("Sending chain to %s. Message id: %s, chain from vault@%d ...", s.Conn().RemotePeer(), uuid, vault.Height)

	payload, err := proto.Marshal(&pb.ChainPayload{
		Vault:        vault,
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

	ok := w.node.sendProtoMessage(s.Conn().RemotePeer(), chainMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	w.requests.Store(req.Header.Uuid, req)
	log.Debugf("chain to: %s was sent. Message Id: %s", s.Conn().RemotePeer(), req.Header.Uuid)
	return true
}

func (w *Wired) onChain(s network.Stream) {
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

	if !w.node.verifyResponse(data) || !w.node.authenticateMessage(s.Conn().RemotePeer(), data) {
		log.Errorf("Failed to authenticate message")
		return
	}

	var payload = &pb.ChainPayload{}
	err = proto.Unmarshal(data.Payload, payload)
	if err != nil {
		log.Error(err)
		return
	}

	remoteID := s.Conn().RemotePeer()
	_ = s.Close()

	if len(payload.Blocks) > 0 {
		log.Debugf("Received chain from %s. Message id:%s. From: %d To: %d LatestHeight: %d.", remoteID, data.Header.Uuid, payload.Blocks[0].GetHeight(), payload.Blocks[len(payload.Blocks)-1].GetHeight(), payload.LatestHeight)
	} else if payload.Vault != nil {
		log.Debugf("Received chain from %s. Message id:%s. Vault@%d only, LatestHeight: %d..", remoteID, data.Header.Uuid, payload.Vault.GetHeight(), payload.LatestHeight)
	}

	w.node.RemoteHeights.Store(remoteID, payload.LatestHeight)

	// init
	if !w.node.isStrictMode && !w.node.isInitialized.Load() {
		c := []ngchain.Item{payload.Vault}
		for i := 0; i < len(payload.Blocks); i++ {
			c = append(c, payload.Blocks[i])
		}

		err = w.node.consensus.InitWithChain(c...)
		if err != nil {
			log.Error(err)
		}

		if w.node.consensus.GetLatestBlockHeight() == payload.LatestHeight {
			w.node.isInitialized.Store(true)
			log.Infof("p2p init finished")
		} else {
			go w.GetChain(remoteID, w.node.consensus.GetLatestVaultHeight()+1)
		}
		return
	}

	localVaultHeight := w.node.consensus.GetLatestVaultHeight()
	if payload.Vault.Height > localVaultHeight {
		//append
		c := []ngchain.Item{payload.Vault}
		for i := 0; i < len(payload.Blocks); i++ {
			c = append(c, payload.Blocks[i])
		}

		err = w.node.consensus.PutNewChain(c...)
		if err != nil {
			log.Error(err)
			return
		}
	} else {
		// forkto
		c := []ngchain.Item{payload.Vault}
		for i := 1; i < len(payload.Blocks); i++ {
			c = append(c, payload.Blocks[i])
		}
		err = w.node.consensus.SwitchTo(c...)
		if err != nil {
			log.Error(err)
			return
		}
	}

	// continue get chain
	if w.node.consensus.GetLatestBlockHeight() < payload.LatestHeight {
		go w.GetChain(remoteID, payload.Vault.Height+1)
		return
	}
}
