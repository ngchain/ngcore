package consensus

import (
	"fmt"
	"github.com/ngchain/ngcore/ngp2p/message"
	"github.com/ngchain/ngcore/ngp2p/wired"

	"github.com/ngchain/ngcore/ngchain"

	core "github.com/libp2p/go-libp2p-core"

	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngtypes"
)

// GetRemoteStatus just get the remote status from remote, and then put it into sync.store
func (mod *syncModule) getRemoteStatus(peerID core.PeerID) error {
	origin := ngchain.GetOriginBlock()
	latest := ngchain.GetLatestBlock()
	cp := ngchain.GetLatestCheckpoint()

	id, stream := ngp2p.GetLocalNode().Ping(peerID, origin.GetHeight(), latest.GetHeight(), cp.Hash(), cp.GetActualDiff().Bytes())
	if stream == nil {
		return fmt.Errorf("failed to send ping, cannot get remote status from %s", peerID)
	}

	reply, err := message.ReceiveReply(id, stream)
	if err != nil {
		return err
	}

	switch reply.Header.MessageType {
	case message.MessageType_PONG:
		pongPayload, err := wired.DecodePongPayload(reply.Payload)
		if err != nil {
			return err
		}

		mod.putRemote(peerID, &remoteRecord{
			id:     peerID,
			origin: pongPayload.Origin,
			latest: pongPayload.Latest,
		})

	case message.MessageType_REJECT:
		return fmt.Errorf("ping is rejected by remote: %s", string(reply.Payload))
	default:
		return fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.MessageType)
	}

	return nil
}

// getRemoteChainFromLocalLatest just get the remote status from remote
func (mod *syncModule) getRemoteChainFromLocalLatest(peerID core.PeerID) (chain []*ngtypes.Block, err error) {
	latestHash := ngchain.GetLatestBlockHash()

	id, s, err := ngp2p.GetLocalNode().GetChain(peerID, [][]byte{latestHash}, nil)
	if s == nil {
		return nil, fmt.Errorf("failed to send getchain: %s", err)
	}

	reply, err := message.ReceiveReply(id, s)
	if err != nil {
		return nil, err
	}

	switch reply.Header.MessageType {
	case message.MessageType_CHAIN:
		chainPayload, err := wired.DecodeChainPayload(reply.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to send ping: %s", err)
		}

		// TODO: add support for hashes etc
		return chainPayload.Blocks, err

	case message.MessageType_REJECT:
		return nil, fmt.Errorf("getchain is rejected by remote: %s", string(reply.Payload))

	case message.MessageType_NOTFOUND:
		return nil, fmt.Errorf("chain is not found in remote")

	default:
		return nil, fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.MessageType)
	}
}

// getRemoteChain just get the remote status from remote
func (mod *syncModule) getRemoteChain(peerID core.PeerID, from [][]byte, to []byte) (chain []*ngtypes.Block, err error) {
	id, s, err := ngp2p.GetLocalNode().GetChain(peerID, from, to)
	if s == nil {
		return nil, fmt.Errorf("failed to send getchain: %s", err)
	}

	reply, err := message.ReceiveReply(id, s)
	if err != nil {
		return nil, err
	}

	switch reply.Header.MessageType {
	case message.MessageType_CHAIN:
		chainPayload, err := wired.DecodeChainPayload(reply.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to send ping: %s", err)
		}

		// TODO: add support for hashes etc
		return chainPayload.Blocks, err

	case message.MessageType_REJECT:
		return nil, fmt.Errorf("getchain is rejected by remote: %s", string(reply.Payload))

	case message.MessageType_NOTFOUND:
		return nil, fmt.Errorf("chain is not found in remote")

	default:
		return nil, fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.MessageType)
	}
}
