package broadcast

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	core "github.com/libp2p/go-libp2p/core"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngtypes"
)

type Broadcast struct {
	PubSub *pubsub.PubSub
	node   core.Host

	ctx    context.Context
	cancel context.CancelFunc

	network       ngtypes.Network
	topics        map[string]*pubsub.Topic
	subscriptions map[string]*pubsub.Subscription

	blockTopic  string
	txTopic     string
	commitTopic string

	OnBlock  chan *ngtypes.FullBlock
	OnTx     chan *ngtypes.FullTx
	OnCommit chan *ngtypes.Commitment

	// OnTxSeen / OnCommitSeen, when set, fire from the pubsub validators
	// for every VALID tx / commitment observed in the flood (including our
	// own publishes). The dandelion router uses them as its "fluff seen"
	// signal to cancel stem fail-safe timers. Must be fast and non-blocking.
	OnTxSeen     func(txHash []byte)
	OnCommitSeen func(commitUnsignedHash []byte)
}

var log = logging.Logger("bcast")

func NewBroadcastProtocol(node core.Host, network ngtypes.Network, blockCh chan *ngtypes.FullBlock, txCh chan *ngtypes.FullTx, commitCh chan *ngtypes.Commitment) *Broadcast {
	var err error

	ctx, cancel := context.WithCancel(context.Background())

	b := &Broadcast{
		PubSub:        nil,
		node:          node,
		ctx:           ctx,
		cancel:        cancel,
		network:       network,
		topics:        make(map[string]*pubsub.Topic),
		subscriptions: make(map[string]*pubsub.Subscription),

		blockTopic:  defaults.GetBroadcastBlockTopic(network),
		txTopic:     defaults.GetBroadcastTxTopic(network),
		commitTopic: defaults.GetBroadcastCommitTopic(network),

		OnBlock:  blockCh,
		OnTx:     txCh,
		OnCommit: commitCh,
	}

	b.PubSub, err = pubsub.NewFloodSub(ctx, node)
	if err != nil {
		panic(err)
	}

	// validate-before-relay: junk must not borrow the network's
	// bandwidth. Everything STATELESS gets checked here (decode, pow,
	// caps, witness root, signature envelopes); stateful checks stay
	// with the pool and the chain
	if err := b.PubSub.RegisterTopicValidator(b.blockTopic, b.validateBlockMsg); err != nil {
		panic(err)
	}
	if err := b.PubSub.RegisterTopicValidator(b.txTopic, b.validateTxMsg); err != nil {
		panic(err)
	}
	if err := b.PubSub.RegisterTopicValidator(b.commitTopic, b.validateCommitMsg); err != nil {
		panic(err)
	}

	b.topics[b.blockTopic], err = b.PubSub.Join(b.blockTopic)
	if err != nil {
		panic(err)
	}

	b.subscriptions[b.blockTopic], err = b.topics[b.blockTopic].Subscribe()
	if err != nil {
		panic(err)
	}

	b.topics[b.txTopic], err = b.PubSub.Join(b.txTopic)
	if err != nil {
		panic(err)
	}

	b.subscriptions[b.txTopic], err = b.topics[b.txTopic].Subscribe()
	if err != nil {
		panic(err)
	}

	b.topics[b.commitTopic], err = b.PubSub.Join(b.commitTopic)
	if err != nil {
		panic(err)
	}

	b.subscriptions[b.commitTopic], err = b.topics[b.commitTopic].Subscribe()
	if err != nil {
		panic(err)
	}

	return b
}

func (b *Broadcast) GoServe() {
	go b.blockListener(b.subscriptions[b.blockTopic])
	go b.txListener(b.subscriptions[b.txTopic])
	go b.commitListener(b.subscriptions[b.commitTopic])
}

func (b *Broadcast) blockListener(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(b.ctx)
		if err != nil {
			if b.ctx.Err() != nil {
				return // shutting down
			}
			log.Error(err)
			continue
		}

		go b.onBroadcastBlock(msg)
	}
}

func (b *Broadcast) txListener(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(b.ctx)
		if err != nil {
			if b.ctx.Err() != nil {
				return // shutting down
			}
			log.Error(err)
			continue
		}

		go b.onBroadcastTx(msg)
	}
}

func (b *Broadcast) commitListener(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(b.ctx)
		if err != nil {
			if b.ctx.Err() != nil {
				return // shutting down
			}
			log.Error(err)
			continue
		}

		go b.onBroadcastCommitment(msg)
	}
}

// Close stops the listeners and leaves all topics
func (b *Broadcast) Close() {
	b.cancel()

	for _, sub := range b.subscriptions {
		sub.Cancel()
	}
	for _, topic := range b.topics {
		_ = topic.Close()
	}
}
