package ngp2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type broadcastProtocol struct {
	PubSub *pubsub.PubSub
	node   *LocalNode

	topics        map[string]*pubsub.Topic
	subscriptions map[string]*pubsub.Subscription
}

func registerBroadcaster(node *LocalNode) *broadcastProtocol {
	var err error

	b := &broadcastProtocol{
		PubSub:        nil,
		node:          node,
		topics:        make(map[string]*pubsub.Topic),
		subscriptions: make(map[string]*pubsub.Subscription),
	}

	b.PubSub, err = pubsub.NewFloodSub(context.Background(), node)
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

	go b.blockListener(b.subscriptions[broadcastBlockTopic])
	go b.txListener(b.subscriptions[broadcastTxTopic])

	return b
}

func (b *broadcastProtocol) blockListener(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(context.Background())
		if err != nil {
			log.Error(err)
			continue
		}

		go b.onBroadcastBlock(msg)
	}
}

func (b *broadcastProtocol) txListener(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(context.Background())
		if err != nil {
			log.Error(err)
			continue
		}

		go b.onBroadcastTx(msg)
	}
}
