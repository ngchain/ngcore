package ngp2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/ngchain/ngcore/ngtypes"
)

func (b *broadcaster) broadcastBlock(block *ngtypes.Block) bool {
	broadcastBlockPayload := block

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
	var broadcastBlockPayload = new(ngtypes.Block)
	err := broadcastBlockPayload.Unmarshal(msg.Data)
	if err != nil {
		log.Error(err)
		return
	}

	log.Debugf("received a new block broadcast@%d", broadcastBlockPayload.GetHeight())
	err = b.node.consensus.PutNewBlock(broadcastBlockPayload)
	if err != nil {
		log.Error(err)
		return
	}

}
