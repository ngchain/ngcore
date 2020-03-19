package ngp2p

import (
	"context"
	"github.com/libp2p/go-libp2p-pubsub"
)

func (b *Broadcaster) sendBroadcastTx() {
	ctx := context.Background()

	var raw []byte
	// TODO
	err := b.topics[broadcastTxTopic].Publish(ctx, raw)
	if err != nil {
		log.Error(err)
	}

}

func (b *Broadcaster) onBroadcastTx(msg *pubsub.Message) {

}
