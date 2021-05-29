package broadcast

import (
	"context"
	"github.com/ngchain/ngcore/ngtypes/ngproto"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngtypes"
)

func (b *Broadcast) BroadcastTx(tx *ngtypes.Tx) error {
	log.Debugf("broadcasting tx %s", tx.BS58())

	raw, err := proto.Marshal(tx.GetProto())
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
	var protoTx ngproto.Tx

	err := proto.Unmarshal(msg.Data, &protoTx)
	if err != nil {
		log.Error(err)
		return
	}

	b.OnTx <- ngtypes.NewTxFromProto(&protoTx)
}
