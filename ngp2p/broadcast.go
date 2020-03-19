package ngp2p

import (
	"context"
	"github.com/libp2p/go-libp2p-pubsub"
)

const broadcastBlockTopic = "/ngp2p/broadcast/block/0.0.1"
const broadcastTxTopic = "/ngp2p/broadcast/tx/0.0.1"

type Broadcaster struct {
	PubSub *pubsub.PubSub
	node   *LocalNode

	topics        map[string]*pubsub.Topic
	subscriptions map[string]*pubsub.Subscription
}

func registerPubSub(ctx context.Context, node *LocalNode) *Broadcaster {
	var err error
	b := &Broadcaster{
		PubSub: nil,
		node:   nil,
		topics: nil,
	}

	b.PubSub, err = pubsub.NewGossipSub(ctx, node)
	if err != nil {
		panic(err)
	}

	b.topics[broadcastBlockTopic], err = b.PubSub.Join(broadcastBlockTopic)
	if err != nil {
		panic(err)
	}

	b.subscriptions[broadcastBlockTopic], err = b.topics[broadcastBlockTopic].Subscribe()
	if err != nil {
		panic(err)
	}

	b.topics[broadcastTxTopic], err = b.PubSub.Join(broadcastTxTopic)
	if err != nil {
		panic(err)
	}

	b.subscriptions[broadcastTxTopic], err = b.topics[broadcastTxTopic].Subscribe()
	if err != nil {
		panic(err)
	}

	go b.bcastBlockHandler(ctx, b.subscriptions[broadcastBlockTopic])
	go b.bcastTxHandler(ctx, b.subscriptions[broadcastTxTopic])

	return b
}

func (b *Broadcaster) bcastBlockHandler(ctx context.Context, sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			log.Error(err)
			continue
		}

		go b.onBroadcastBlock(msg)
	}
}

func (b *Broadcaster) bcastTxHandler(ctx context.Context, sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			log.Error(err)
			continue
		}

		go b.onBroadcastTx(msg)
	}
}
