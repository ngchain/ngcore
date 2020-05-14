package consensus

import (
	"fmt"

	core "github.com/libp2p/go-libp2p-core"

	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngtypes"
)

// getRemoteChain just get the remote status from remote
func (sync *syncModule) getRemoteChain(peerID core.PeerID) (chain []*ngtypes.Block, err error) {
	latestHash := pow.chain.GetLatestBlockHash()

	id, s := pow.localNode.GetChain(peerID, [][]byte{latestHash}, nil)
	if s == nil {
		return nil, fmt.Errorf("failed to send getchain")
	}

	reply, err := ngp2p.ReceiveReply(id, s)
	if err != nil {
		return nil, err
	}

	switch reply.Header.MessageType {
	case ngp2p.MessageType_CHAIN:
		chainPayload, err := ngp2p.DecodeChainPayload(reply.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to send ping: %s", err)
		}

		// TODO: add support for hashes etc
		return chainPayload.Blocks, err

	case ngp2p.MessageType_REJECT:
		return nil, fmt.Errorf("getchain is rejected by remote")

	case ngp2p.MessageType_NOTFOUND:
		return nil, fmt.Errorf("chain is not found in remote")

	default:
		return nil, fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.MessageType)
	}
}
