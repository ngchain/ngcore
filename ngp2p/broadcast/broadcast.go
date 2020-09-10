package broadcast

import (
	"context"
	logging "github.com/ipfs/go-log/v2"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngtypes"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type Broadcast struct {
	PubSub *pubsub.PubSub
	node   core.Host

	topics        map[string]*pubsub.Topic
	subscriptions map[string]*pubsub.Subscription

	OnBlock chan *ngtypes.Block
	OnTx    chan *ngtypes.Tx
}

var log = logging.Logger("bcast")

func NewBroadcastProtocol(node core.Host, blockCh chan *ngtypes.Block, txCh chan *ngtypes.Tx) *Broadcast {
	var err error

	b := &Broadcast{
		PubSub:        nil,
		node:          node,
		topics:        make(map[string]*pubsub.Topic),
		subscriptions: make(map[string]*pubsub.Subscription),
		OnBlock:       blockCh,
		OnTx:          txCh,
	}

	b.PubSub, err = pubsub.NewFloodSub(context.Background(), node)
	if err != nil {
		panic(err)
	}

	b.topics[defaults.BroadcastBlockTopic], err = b.PubSub.Join(defaults.BroadcastBlockTopic)
	if err != nil {
		panic(err)
	}

	b.subscriptions[defaults.BroadcastBlockTopic], err = b.topics[defaults.BroadcastBlockTopic].Subscribe()
	if err != nil {
		panic(err)
	}

	b.topics[defaults.BroadcastTxTopic], err = b.PubSub.Join(defaults.BroadcastTxTopic)
	if err != nil {
		panic(err)
	}

	b.subscriptions[defaults.BroadcastTxTopic], err = b.topics[defaults.BroadcastTxTopic].Subscribe()
	if err != nil {
		panic(err)
	}

	go b.blockListener(b.subscriptions[defaults.BroadcastBlockTopic])
	go b.txListener(b.subscriptions[defaults.BroadcastTxTopic])

	return b
}

func (b *Broadcast) blockListener(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(context.Background())
		if err != nil {
			log.Error(err)
			continue
		}

		go b.onBroadcastBlock(msg)
	}
}

func (b *Broadcast) txListener(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(context.Background())
		if err != nil {
			log.Error(err)
			continue
		}

		go b.onBroadcastTx(msg)
	}
}
