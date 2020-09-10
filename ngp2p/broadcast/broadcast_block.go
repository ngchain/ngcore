package broadcast

import (
	"context"
	"fmt"
	"github.com/ngchain/ngcore/ngp2p/defaults"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (b *Broadcast) BroadcastBlock(block *ngtypes.Block) error {
	broadcastBlockPayload := block

	raw, err := utils.Proto.Marshal(broadcastBlockPayload)
	if err != nil {
		return fmt.Errorf("failed to sign pb data")
	}

	err = b.topics[defaults.BroadcastBlockTopic].Publish(context.Background(), raw)
	if err != nil {
		return err
	}

	log.Debugf("broadcast block@%d: %x", block.GetHeight(), block.Hash())

	return nil
}

func (b *Broadcast) onBroadcastBlock(msg *pubsub.Message) {
	var broadcastBlockPayload = new(ngtypes.Block)

	err := utils.Proto.Unmarshal(msg.Data, broadcastBlockPayload)
	if err != nil {
		log.Error(err)
		return
	}

	newBlock := broadcastBlockPayload
	log.Debugf("received a new block broadcast@%d", newBlock.GetHeight())

	b.OnBlock <- newBlock
}
