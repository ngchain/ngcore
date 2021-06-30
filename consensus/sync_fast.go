package consensus

import (
	"encoding/binary"
	"fmt"

	"github.com/ngchain/ngcore/ngp2p/message"
	"github.com/ngchain/ngcore/ngp2p/wired"
	"github.com/ngchain/ngcore/ngtypes"
)

// convert local origin to remote checkpoint
func (mod *syncModule) switchToRemoteCheckpoint(record *RemoteRecord) error {
	if mod.Locker.IsLocked() {
		return nil
	}

	mod.Locker.Lock()
	defer mod.Locker.Unlock()

	log.Warnf("start syncing with remote node %s, target height %d", record.id, record.latest)

	// get chain
	remoteCheckpoint, err := mod.getRemoteCheckpoint(record)
	if err != nil {
		return err
	}

	err = mod.pow.Chain.InitFromCheckpoint(remoteCheckpoint)
	if err != nil {
		return err
	}

	return nil
}

func (mod *syncModule) getRemoteCheckpoint(record *RemoteRecord) (*ngtypes.Block, error) {
	to := make([]byte, 16)

	if record.checkpointHeight <= 2*ngtypes.BlockCheckRound {
		return ngtypes.GetGenesisBlock(mod.pow.Network), nil
	}

	checkpointHeight := record.checkpointHeight - 2*ngtypes.BlockCheckRound
	binary.LittleEndian.PutUint64(to[0:], checkpointHeight)
	binary.LittleEndian.PutUint64(to[8:], checkpointHeight)
	id, s, err := mod.localNode.SendGetChain(record.id, nil, to) // nil means get MaxBlocks number blocks
	if err != nil {
		return nil, err
	}

	reply, err := wired.ReceiveReply(id, s)
	if err != nil {
		return nil, err
	}

	switch reply.Header.MessageType {
	case message.MessageType_CHAIN:
		chainPayload, err := wired.DecodeChainPayload(reply.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to send ping: %s", err)
		}

		if len(chainPayload.Blocks) != 1 {
			log.Debugf("%#v", chainPayload.Blocks)
			return nil, fmt.Errorf("invalid blocks payload length: should be 1 but got %d", len(chainPayload.Blocks))
		}

		//checkpoint := chainPayload.Blocks[0]
		//if !bytes.Equal(checkpoint.Hash(), record.checkpointHash) {
		//	return nil, fmt.Errorf("invalid checkpoint: should be %x, but got %x", record.checkpointHash, checkpoint.Hash())
		//}

		return chainPayload.Blocks[0], err

	case message.MessageType_REJECT:
		return nil, fmt.Errorf("getchain is rejected by remote: %s", string(reply.Payload))

	case message.MessageType_NOTFOUND:
		return nil, fmt.Errorf("chain is not found in remote")

	default:
		return nil, fmt.Errorf("remote replies ping with invalid messgae type: %s", reply.Header.MessageType)
	}
}
