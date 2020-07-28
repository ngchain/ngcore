package ngp2p

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (b *broadcastProtocol) BroadcastBlock(block *ngtypes.Block) error {
	broadcastBlockPayload := block

	raw, err := utils.Proto.Marshal(broadcastBlockPayload)
	if err != nil {
		return fmt.Errorf("failed to sign pb data")
	}

	err = b.topics[broadcastBlockTopic].Publish(context.Background(), raw)
	if err != nil {
		return err
	}

	log.Debugf("broadcasted block@%d: %x", block.GetHeight(), block.Hash())

	return nil
}

func (b *broadcastProtocol) onBroadcastBlock(msg *pubsub.Message) {
	var broadcastBlockPayload = new(ngtypes.Block)

	err := utils.Proto.Unmarshal(msg.Data, broadcastBlockPayload)
	if err != nil {
		log.Error(err)
		return
	}

	newBlock := broadcastBlockPayload
	log.Debugf("received a new block broadcast@%d", newBlock.GetHeight())

	b.node.OnBlock <- newBlock
}
