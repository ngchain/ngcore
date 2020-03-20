package ngp2p

import (
	"context"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/ngin-network/ngcore/ngp2p/pb"
	"github.com/ngin-network/ngcore/ngtypes"
)

func (b *Broadcaster) broadcastBlock(block *ngtypes.Block, vault *ngtypes.Vault) bool {
	broadcastBlockPayload := &pb.BroadcastBlockPayload{
		Vault: vault,
		Block: block,
	}

	payload, err := broadcastBlockPayload.Marshal()
	if err != nil {
		log.Errorf("failed to sign pb data")
		return false
	}

	// create message data
	req := &pb.Message{
		Header:  b.node.NewHeader(uuid.New().String()),
		Payload: payload,
	}

	raw, err := req.Marshal()
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

func (b *Broadcaster) onBroadcastBlock(msg *pubsub.Message) {
	raw := msg.Data
	var message = &pb.Message{}
	err := message.Unmarshal(raw)
	if err != nil {
		log.Error(err)
		return
	}

	var broadcastBlockPayload = &pb.BroadcastBlockPayload{}
	err = broadcastBlockPayload.Unmarshal(message.Payload)
	if err != nil {
		log.Error(err)
		return
	}

	if broadcastBlockPayload.Vault != nil {
		err := b.node.Chain.PutNewBlockWithVault(broadcastBlockPayload.Vault, broadcastBlockPayload.Block)
		if err != nil {
			log.Error(err)
			return
		}
	} else {
		err = b.node.Chain.PutNewBlock(broadcastBlockPayload.Block)
		if err != nil {
			log.Error(err)
			return
		}
	}
}
