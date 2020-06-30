package ngp2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (b *broadcastProtocol) BroadcastTx(tx *ngtypes.Tx) bool {
	log.Debugf("broadcasting tx %s", tx.BS58())

	raw, err := utils.Proto.Marshal(tx)
	if err != nil {
		log.Errorf("failed to sign pb data")
		return false
	}

	err = b.topics[broadcastTxTopic].Publish(context.Background(), raw)
	if err != nil {
		log.Error(err)
		return false
	}

	log.Debugf("broadcasted Tx:%s", tx.ID())

	return true
}

func (b *broadcastProtocol) onBroadcastTx(msg *pubsub.Message) {
	var tx = &ngtypes.Tx{}

	err := utils.Proto.Unmarshal(msg.Data, tx)
	if err != nil {
		log.Error(err)
		return
	}

	b.node.OnTx <- tx
}
