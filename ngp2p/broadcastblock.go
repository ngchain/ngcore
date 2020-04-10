package ngp2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngtypes"
)

func (b *broadcaster) broadcastBlock(block *ngtypes.Block, vault *ngtypes.Vault) bool {
	broadcastBlockPayload := &pb.BroadcastBlockPayload{
		Vault: vault,
		Block: block,
	}

	raw, err := broadcastBlockPayload.Marshal()
	if err != nil {
		log.Errorf("failed to sign pb data")
		return false
	}

	err = b.topics[broadcastBlockTopic].Publish(context.Background(), raw)
	if err != nil {
		log.Error(err)
		return false
	}

	return true
}

func (b *broadcaster) onBroadcastBlock(msg *pubsub.Message) {
	var broadcastBlockPayload = &pb.BroadcastBlockPayload{}
	err := broadcastBlockPayload.Unmarshal(msg.Data)
	if err != nil {
		log.Error(err)
		return
	}

	if broadcastBlockPayload.Vault != nil {
		log.Debugf("received a new block broadcast@%d with vault@%d", broadcastBlockPayload.Block.GetHeight(), broadcastBlockPayload.Vault.GetHeight())
		err := b.node.consensus.PutNewBlockWithVault(broadcastBlockPayload.Vault, broadcastBlockPayload.Block)
		if err != nil {
			log.Error(err)
			return
		}
	} else {
		log.Debugf("received a new block broadcast@%d", broadcastBlockPayload.Block.GetHeight())
		err = b.node.consensus.PutNewBlock(broadcastBlockPayload.Block)
		if err != nil {
			log.Error(err)
			return
		}
	}
}
