package ngp2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const broadcastBlockTopic = "/ngp2p/broadcast/block/0.0.1"
const broadcastTxTopic = "/ngp2p/broadcast/tx/0.0.1"

type broadcaster struct {
	PubSub *pubsub.PubSub
	node   *LocalNode

	topics        map[string]*pubsub.Topic
	subscriptions map[string]*pubsub.Subscription
}

func registerBroadcaster(node *LocalNode) *broadcaster {
	var err error

	b := &broadcaster{
		PubSub:        nil,
		node:          node,
		topics:        make(map[string]*pubsub.Topic),
		subscriptions: make(map[string]*pubsub.Subscription),
	}

	b.PubSub, err = pubsub.NewGossipSub(context.Background(), node)
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
	go func() {
		for {
			select {
			case block := <-b.node.consensus.MinedBlockToP2PCh:
				b.broadcastBlock(block)

			case tx := <-b.node.consensus.NewCreatedTxEvent:
				b.broadcastTx(tx)
			}
		}
	}()

	return b
}

func (b *broadcaster) blockListener(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(context.Background())
		if msg.GetFrom() == b.node.ID() {
			continue
		}
		if err != nil {
			log.Error(err)
			continue
		}

		go b.onBroadcastBlock(msg)
	}
}

func (b *broadcaster) txListener(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(context.Background())
		if msg.GetFrom() == b.node.ID() {
			continue
		}
		if err != nil {
			log.Error(err)
			continue
		}

		go b.onBroadcastTx(msg)
	}
}
