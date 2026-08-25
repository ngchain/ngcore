package broadcast

import (
	"context"

	"github.com/c0mm4nd/rlp"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/ngchain/ngcore/ngtypes"
)

// BroadcastCommitment gossips a blind commitment to the network, mirroring
// BroadcastTx.
func (b *Broadcast) BroadcastCommitment(commit *ngtypes.Commitment) error {
	log.Debugf("broadcasting commitment %x", commit.Hash)

	raw, err := rlp.EncodeToBytes(commit)
	if err != nil {
		log.Errorf("failed to encode commitment")
		return err
	}

	err = b.topics[b.commitTopic].Publish(context.Background(), raw)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Debugf("broadcast commitment: %x", commit.Hash)

	return nil
}

func (b *Broadcast) onBroadcastCommitment(msg *pubsub.Message) {
	var newCommit ngtypes.Commitment

	err := rlp.DecodeBytes(msg.Data, &newCommit)
	if err != nil {
		log.Error(err)
		return
	}

	b.OnCommit <- &newCommit
}
