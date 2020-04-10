package ngp2p

import (
	"io/ioutil"

	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
)

func (w *wired) GetChain(peerID peer.ID, requestHeight uint64) bool {
	payload, err := proto.Marshal(&pb.GetChainPayload{
		VaultHeight: requestHeight,
	})
	if err != nil {
		log.Error("failed to sign pb data")
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
		log.Error("failed to sign pb data")
		return false
	}

	// add the signature to the message
	req.Header.Sign = signature

	ok := w.node.sendProtoMessage(peerID, getChainMethod, req)
	if !ok {
		return false
	}

	// store ref request so response handler has access to it
	w.requests.Store(req.Header.Uuid, req)
	log.Debugf("getchain to: %s was sent. Message Id: %s, request vault height: %d", peerID, req.Header.Uuid, requestHeight)
	return true
}

func (w *wired) onGetChain(s network.Stream) {
	// get request data
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

	if !w.node.authenticateMessage(s.Conn().RemotePeer(), data) {
		log.Errorf("Failed to authenticate message")
		return
	}

	var getchain = &pb.GetChainPayload{}
	err = proto.Unmarshal(data.Payload, getchain)
	if err != nil {
		log.Error(err)
		return
	}

	remoteID := s.Conn().RemotePeer()
	_ = s.Close()

	log.Debugf("Received getchain request from %s. Requested vault@%d", remoteID, getchain.VaultHeight)

	// chain
	localHeight := w.node.consensus.GetLatestBlockHeight()
	if localHeight < getchain.VaultHeight {
		go w.Reject(remoteID, data.Header.Uuid)
		return
	}

	var blocks = make([]*ngtypes.Block, 0, ngtypes.BlockCheckRound)

	for i := getchain.VaultHeight * ngtypes.BlockCheckRound; i < (getchain.VaultHeight+1)*ngtypes.BlockCheckRound; i++ {
		b, err := w.node.consensus.GetBlockByHeight(i)
		if err != nil {
			log.Errorf("missing block@%d: %s", i, err)
			break
		}

		if b == nil {
			log.Errorf("missing block@%d", i)
			break
		} else {
			blocks = append(blocks, b)
		}
	}

	vault, err := w.node.consensus.GetVaultByHeight(getchain.VaultHeight)
	if err != nil {
		log.Errorf("failed to get vault")
		go w.NotFound(remoteID, data.Header.Uuid)
		return
	}

	go w.Chain(remoteID, data.Header.Uuid, vault, blocks...)
}
