package broadcast

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/ngcore/ngtypes"
)

// maxTxWireSize bounds a gossiped tx message: the largest legitimate
// tx is extra (1 MiB) plus the biggest envelope, with rlp overhead
const maxTxWireSize = ngtypes.TxMaxExtraSize + 64<<10

// validateBlockMsg gates block gossip: only structurally valid,
// pow-sealed, capacity-respecting blocks relay further. All checks
// here are STATELESS — CheckError covers pow, caps, tries and the
// witness commitment
func (b *Broadcast) validateBlockMsg(_ context.Context, _ peer.ID, msg *pubsub.Message) bool {
	if len(msg.Data) > ngtypes.MaxBlockBytes {
		return false
	}

	var block ngtypes.FullBlock
	if err := rlp.DecodeBytes(msg.Data, &block); err != nil {
		return false
	}

	if block.Network != b.network {
		return false
	}

	return block.CheckError() == nil
}

// validateTxMsg gates tx gossip: junk and unverifiable envelopes stop
// at the first hop. Full and recover envelopes verify statelessly;
// compact ones (which need the on-chain key registry) pass through
// bounded in size, for the pool to judge
func (b *Broadcast) validateTxMsg(_ context.Context, _ peer.ID, msg *pubsub.Message) bool {
	if len(msg.Data) > maxTxWireSize {
		return false
	}

	var tx ngtypes.FullTx
	if err := rlp.DecodeBytes(msg.Data, &tx); err != nil {
		return false
	}

	if tx.Network != b.network || !tx.IsSigned() {
		return false
	}

	if tx.IsCompactEnvelope() {
		// stateful: the pool verifies against the key registry
		b.notifyTxSeen(&tx)
		return true
	}

	if tx.Verify(nil) != nil {
		return false
	}

	b.notifyTxSeen(&tx)
	return true
}

// notifyTxSeen feeds the dandelion router's fluff-seen set: only txs that
// passed the validator (and thus actually relay) count as fluffed.
func (b *Broadcast) notifyTxSeen(tx *ngtypes.FullTx) {
	if b.OnTxSeen != nil {
		b.OnTxSeen(tx.GetHash())
	}
}

// maxCommitWireSize bounds a gossiped commitment: a 32B hash plus the biggest
// signature envelope, with rlp overhead
const maxCommitWireSize = 64 << 10

// validateCommitMsg gates commitment gossip: junk and unverifiable envelopes
// stop at the first hop. Full and recover envelopes verify statelessly;
// compact ones (which need the on-chain key registry) pass through bounded in
// size, for the pool to judge
func (b *Broadcast) validateCommitMsg(_ context.Context, _ peer.ID, msg *pubsub.Message) bool {
	if len(msg.Data) > maxCommitWireSize {
		return false
	}

	var commit ngtypes.Commitment
	if err := rlp.DecodeBytes(msg.Data, &commit); err != nil {
		return false
	}

	if commit.Network != b.network || !commit.IsSigned() || len(commit.Hash) != ngtypes.HashSize {
		return false
	}

	if len(commit.Sign) > 0 && commit.Sign[0] == 0x02 {
		// compact envelope: the pool verifies against the key registry
		b.notifyCommitSeen(&commit)
		return true
	}

	if commit.Verify(nil) != nil {
		return false
	}

	b.notifyCommitSeen(&commit)
	return true
}

// notifyCommitSeen is notifyTxSeen for commitments, keyed by the unsigned
// hash (unique per commitment content).
func (b *Broadcast) notifyCommitSeen(commit *ngtypes.Commitment) {
	if b.OnCommitSeen != nil {
		b.OnCommitSeen(commit.GetUnsignedHash())
	}
}
