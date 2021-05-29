package broadcast

import (
	"context"
	"fmt"
	"github.com/ngchain/ngcore/ngtypes/ngproto"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngtypes"
)

func (b *Broadcast) BroadcastBlock(block *ngtypes.Block) error {
	raw, err := proto.Marshal(block.GetProto())
	if err != nil {
		return fmt.Errorf("failed to sign pb data")
	}

	err = b.topics[b.blockTopic].Publish(context.Background(), raw)
	if err != nil {
		return err
	}

	log.Debugf("broadcast block@%d: %x", block.Header.GetHeight(), block.GetHash())

	return nil
}

func (b *Broadcast) onBroadcastBlock(msg *pubsub.Message) {
	var protoBlock = new(ngproto.Block)

	err := proto.Unmarshal(msg.Data, protoBlock)
	if err != nil {
		log.Error(err)
		return
	}

	newBlock := ngtypes.NewBlockFromProto(protoBlock)
	log.Debugf("received a new block broadcast@%d", newBlock.Header.GetHeight())

	b.OnBlock <- newBlock
}
