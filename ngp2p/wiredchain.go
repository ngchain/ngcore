package ngp2p

import (
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngin-network/ngcore/chain"
	"github.com/ngin-network/ngcore/ngp2p/pb"
	"github.com/ngin-network/ngcore/ngtypes"
	"io/ioutil"
)

func (w *Wired) Chain(s network.Stream, uuid string, getchain *pb.GetChainPayload) bool {
	log.Infof("Sending Chain to %s. Message id: %s, Chain from vault@%d ...", s.Conn().RemotePeer(), uuid, getchain.VaultHeight)
	var blocks = make([]*ngtypes.Block, 0, ngtypes.BlockCheckRound)

	for i := getchain.VaultHeight * ngtypes.BlockCheckRound; i < (getchain.VaultHeight+1)*ngtypes.BlockCheckRound; i++ {
		b, err := w.node.Chain.GetBlockByHeight(i)
		if err != nil {

		}
		if b == nil {
			log.Errorf("missing block@%d", i)
			return false
		}
		blocks = append(blocks, b)
	}

	vault, err := w.node.Chain.GetVaultByHeight(getchain.VaultHeight)
	if err != nil {
		log.Errorf("failed to get vault")
	}
	payload, err := proto.Marshal(&pb.ChainPayload{
		Vault:        vault,
		Blocks:       blocks,
		LatestHeight: w.node.Chain.GetLatestBlockHeight(),
	})
	if err != nil {
		log.Infof("failed to sign pb data")
		return false
	}

	// create message data
	req := &pb.Message{
		Header:  w.node.NewHeader(uuid),
		Payload: payload,
	}

	// sign the data
	signature, err := w.node.signProtoMessage(req)
	if err != nil {
		log.Infof("failed to sign pb data")
		return false
	}

	// add the signature to the message
	req.Header.Sign = signature

	ok := w.node.sendProtoMessage(s.Conn().RemotePeer(), chainMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	w.requests[req.Header.Uuid] = req
	log.Infof("Chain to: %s was sent. Message Id: %s", s.Conn().RemotePeer(), req.Header.Uuid)
	return true
}

func (w *Wired) onChain(s network.Stream) {
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

	if !w.node.verifyResponse(data.Header) {
		log.Errorf("Failed to authenticate message")
		return
	}

	var payload = &pb.ChainPayload{}
	err = proto.Unmarshal(data.Payload, payload)
	if err != nil {
		log.Error(err)
		return
	}

	log.Infof("Received Chain from %s. Message id:%s. From: %d To: %d LatestHeight: %d.", s.Conn().RemotePeer(), data.Header.Uuid, payload.Blocks[0].GetHeight(), payload.Blocks[len(payload.Blocks)-1].GetHeight(), payload.LatestHeight)
	w.node.RemoteHeights.Store(s.Conn().RemotePeer(), payload.LatestHeight)

	localVaultHeight := w.node.Chain.GetLatestVaultHeight()
	if payload.Vault.Height > localVaultHeight {
		//append
		err = w.node.Chain.PutNewBlockWithVault(payload.Vault, payload.Blocks[0])
		if err != nil {
			log.Error(err)
			return
		}
		for i := 1; i < len(payload.Blocks); i++ {
			err := w.node.Chain.PutNewBlock(payload.Blocks[i])
			if err != nil {
				log.Error(err)
				return
			}
		}
	} else {
		//forkto
		c := make([]chain.Item, 1, ngtypes.BlockCheckRound+1)
		c[0] = payload.Vault
		for i := 1; i < len(payload.Blocks); i++ {
			c = append(c, payload.Blocks[i])
		}
		err := w.node.Chain.SwitchTo(c...)
		if err != nil {
			log.Error(err)
			return
		}
	}

	// continue get chain
	if w.node.Chain.GetLatestBlockHeight()+ngtypes.BlockCheckRound < payload.LatestHeight {
		w.GetChain(s.Conn().RemotePeer(), payload.Vault.Height+1)
	}
}
