package broadcast

import (
	"context"

	"github.com/c0mm4nd/rlp"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/ngchain/ngcore/ngtypes"
)

func (b *Broadcast) BroadcastTx(tx *ngtypes.Tx) error {
	log.Debugf("broadcasting tx %s", tx.BS58())

	raw, err := rlp.EncodeToBytes(tx)
	if err != nil {
		log.Errorf("failed to sign pb data")
		return err
	}

	err = b.topics[b.txTopic].Publish(context.Background(), raw)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Debugf("broadcast Tx: %s", tx.ID())

	return nil
}

func (b *Broadcast) onBroadcastTx(msg *pubsub.Message) {
	var newTx ngtypes.Tx

	err := rlp.DecodeBytes(msg.Data, &newTx)
	if err != nil {
		log.Error(err)
		return
	}

	b.OnTx <- &newTx
}
