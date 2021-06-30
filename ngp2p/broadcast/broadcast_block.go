package broadcast

import (
	"context"
	"fmt"
	"github.com/c0mm4nd/rlp"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/ngchain/ngcore/ngtypes"
)

func (b *Broadcast) BroadcastBlock(block *ngtypes.Block) error {
	raw, err := rlp.EncodeToBytes(block)
	if err != nil {
		return fmt.Errorf("failed to sign pb data")
	}

	err = b.topics[b.blockTopic].Publish(context.Background(), raw)
	if err != nil {
		return err
	}

	log.Debugf("broadcast block@%d: %x", block.Header.Height, block.GetHash())

	return nil
}

func (b *Broadcast) onBroadcastBlock(msg *pubsub.Message) {
	var newBlock ngtypes.Block

	err := rlp.DecodeBytes(msg.Data, &newBlock)
	if err != nil {
		log.Error(err)
		return
	}

	log.Debugf("received a new block broadcast@%d", newBlock.Header.Height)

	b.OnBlock <- &newBlock
}
