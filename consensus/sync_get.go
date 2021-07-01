package consensus

import (
	"fmt"

	"github.com/ngchain/ngcore/ngp2p/wired"

	core "github.com/libp2p/go-libp2p-core"

	"github.com/ngchain/ngcore/ngtypes"
)

// GetRemoteStatus just get the remote status from remote, and then put it into sync.store
func (mod *syncModule) getRemoteStatus(peerID core.PeerID) error {
	origin := mod.pow.Chain.GetOriginBlock()
	latest := mod.pow.Chain.GetLatestBlock()
	cp := mod.pow.Chain.GetLatestCheckpoint()

	id, stream := mod.localNode.SendPing(peerID, origin.Header.Height, latest.Header.Height, cp.GetHash(), cp.GetActualDiff().Bytes())
	if stream == nil {
		log.Infof("failed to send ping, cannot get remote status from %s", peerID) // level down this
		return nil
	}

	reply, err := wired.ReceiveReply(id, stream)
	if err != nil {
		return err
	}

	switch reply.Header.Type {
	case wired.PongMsg:
		pongPayload, err := wired.DecodePongPayload(reply.Payload)
		if err != nil {
			return err
		}

		if _, exists := mod.store[peerID]; !exists {
			mod.putRemote(peerID, NewRemoteRecord(peerID, pongPayload.Origin, pongPayload.Latest,
				pongPayload.CheckpointHash, pongPayload.CheckpointDiff))
		} else {
			mod.store[peerID].update(pongPayload.Origin, pongPayload.Latest,
				pongPayload.CheckpointHash, pongPayload.CheckpointDiff)
		}

	case wired.RejectMsg:
		return fmt.Errorf("ping is rejected by remote: %s", string(reply.Payload))
	default:
		return fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.Type)
	}

	return nil
}

// getRemoteChainFromLocalLatest just get the remote status from remote
func (mod *syncModule) getRemoteChainFromLocalLatest(record *RemoteRecord) (chain []*ngtypes.Block, err error) {
	latestHash := mod.pow.Chain.GetLatestBlockHash()

	id, s, err := mod.localNode.SendGetChain(record.id, [][]byte{latestHash}, nil) // nil means get MaxBlocks number blocks
	if s == nil {
		return nil, fmt.Errorf("failed to send getchain: %s", err)
	}

	reply, err := wired.ReceiveReply(id, s)
	if err != nil {
		return nil, err
	}

	switch reply.Header.Type {
	case wired.ChainMsg:
		chainPayload, err := wired.DecodeChainPayload(reply.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to send ping: %s", err)
		}

		// TODO: add support for hashes etc
		return chainPayload.Blocks, err

	case wired.RejectMsg:
		return nil, fmt.Errorf("getchain is rejected by remote: %s", string(reply.Payload))

	default:
		return nil, fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.Type)
	}
}

// getRemoteChain get the chain from remote node
func (mod *syncModule) getRemoteChain(peerID core.PeerID, from [][]byte, to []byte) (chain []*ngtypes.Block, err error) {
	id, s, err := mod.localNode.SendGetChain(peerID, from, to)
	if s == nil {
		return nil, fmt.Errorf("failed to send getchain: %s", err)
	}

	reply, err := wired.ReceiveReply(id, s)
	if err != nil {
		return nil, err
	}

	switch reply.Header.Type {
	case wired.ChainMsg:
		chainPayload, err := wired.DecodeChainPayload(reply.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to send ping: %s", err)
		}

		// TODO: add support for hashes etc
		return chainPayload.Blocks, err

	case wired.RejectMsg:
		return nil, fmt.Errorf("getchain is rejected by remote: %s", string(reply.Payload))

	default:
		return nil, fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.Type)
	}
}

func (mod *syncModule) getRemoteStateSheet(record *RemoteRecord) (sheet *ngtypes.Sheet, err error) {
	id, s, err := mod.localNode.SendGetSheet(record.id, record.checkpointHeight, record.checkpointHash)
	if s == nil {
		return nil, fmt.Errorf("failed to send getsheet: %s", err)
	}

	reply, err := wired.ReceiveReply(id, s)
	if err != nil {
		return nil, err
	}

	switch reply.Header.Type {
	case wired.SheetMsg:
		sheetPayload, err := wired.DecodeSheetPayload(reply.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to send ping: %s", err)
		}

		// TODO: add support for hashes etc
		return sheetPayload.Sheet, err

	case wired.RejectMsg:
		return nil, fmt.Errorf("getsheet is rejected by remote: %s", string(reply.Payload))

	default:
		return nil, fmt.Errorf("remote replies with invalid messgae type: %s", reply.Header.Type)
	}
}
