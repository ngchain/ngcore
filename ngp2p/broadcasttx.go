package ngp2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (b *broadcaster) broadcastTx(tx *ngtypes.Tx) bool {
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

	return true
}

func (b *broadcaster) onBroadcastTx(msg *pubsub.Message) {
	var tx = &ngtypes.Tx{}
	err := utils.Proto.Unmarshal(msg.Data, tx)
	if err != nil {
		log.Error(err)
		return
	}

	err = b.node.consensus.CheckTxs(tx)
	if err != nil {
		log.Errorf("failed dealing new tx %s from broadcast: %s", tx.BS58(), err)
		return
	}
	err = b.node.consensus.PutTxs(tx)
	if err != nil {
		log.Error(err)
		return
	}
}
