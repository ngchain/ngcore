package broadcast

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngtypes"
)

func (b *Broadcast) BroadcastBlock(block *ngtypes.Block) error {
	broadcastBlockPayload := block

	raw, err := proto.Marshal(broadcastBlockPayload)
	if err != nil {
		return fmt.Errorf("failed to sign pb data")
	}

	err = b.topics[b.blockTopic].Publish(context.Background(), raw)
	if err != nil {
		return err
	}

	log.Debugf("broadcast block@%d: %x", block.GetHeight(), block.GetHash())

	return nil
}

func (b *Broadcast) onBroadcastBlock(msg *pubsub.Message) {
	var broadcastBlockPayload = new(ngtypes.Block)

	err := proto.Unmarshal(msg.Data, broadcastBlockPayload)
	if err != nil {
		log.Error(err)
		return
	}

	newBlock := broadcastBlockPayload
	log.Debugf("received a new block broadcast@%d", newBlock.GetHeight())

	b.OnBlock <- newBlock
}
