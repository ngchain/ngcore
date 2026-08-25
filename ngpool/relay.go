package ngpool

import (
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

// ErrNotEffectTx is returned when a non effect tx is submitted for relay: only
// Transact/Deploy reveals go through commit-reveal and can be relayed.
var ErrNotEffectTx = errors.New("only Transact/Deploy reveals can be relayed")

// queuedReveal is a signed reveal the node relays on the sender's behalf. It
// carries an expiry: once the next block height passes it, the reveal's commit
// window has closed and the entry is dropped.
type queuedReveal struct {
	tx       *ngtypes.FullTx
	expireAt uint64
}

// RelayReveal accepts a signed effect-tx reveal to relay on the sender's
// behalf. The node holds it privately and, on every tip movement, retargets
// its Height to the new next block and re-submits it — until the reveal lands
// (its commitment is consumed on chain) or its reveal window closes. No
// re-signing is ever needed: an effect tx signs its height-independent
// SigningHash, so one signature is valid across the whole window.
//
// This is the node half of the fire-and-forget commit-reveal flow: the wallet
// signs ONE reveal plus its commitment, hands both to its own node, and may go
// offline. The reveal stays private (only this node holds it) until a tip
// movement makes it admissible, at which point it is gossiped — the intended
// reveal moment.
func (pool *TxPool) RelayReveal(reveal *ngtypes.FullTx) error {
	if !reveal.IsSigned() {
		return ngtypes.ErrTxUnsigned
	}
	if reveal.Type != ngtypes.TransactTx && reveal.Type != ngtypes.DeployTx {
		return ErrNotEffectTx
	}
	if len(reveal.Salt) < ngtypes.MinSaltSize {
		return errors.Errorf("reveal salt is %d bytes, need >= %d", len(reveal.Salt), ngtypes.MinSaltSize)
	}
	from, err := reveal.From()
	if err != nil {
		return err
	}

	next := pool.chain.GetLatestBlockHeight() + 1

	pool.Lock()
	if _, exists := pool.revealQueue[from]; !exists && len(pool.revealQueue) >= pool.MaxSize {
		pool.Unlock()
		return ErrPoolFull
	}
	// the commitment rides the next block (height `next`); the reveal is then
	// admissible in (next, next+CommitWindow], so the entry expires past that
	pool.revealQueue[from] = &queuedReveal{tx: reveal, expireAt: next + ngtypes.CommitWindow}
	pool.Unlock()

	// try once now — the commitment may already be on chain — then rely on
	// tip movements to retry across the window
	pool.relayReveals()
	return nil
}

// relayReveals retargets every queued reveal to the current next height and
// re-attempts admission, dropping the ones whose window has closed. It never
// holds the pool lock across PutTx (which takes it) and does its own db reads,
// so it is safe to run from a goroutine off the tip-changed hook. Each attempt
// works on a COPY of the reveal (only Height differs), so the shared queued
// template is never mutated and concurrent drains cannot race on it.
func (pool *TxPool) relayReveals() {
	next := pool.chain.GetLatestBlockHeight() + 1

	pool.Lock()
	templates := make([]*ngtypes.FullTx, 0, len(pool.revealQueue))
	for from, q := range pool.revealQueue {
		if next > q.expireAt {
			delete(pool.revealQueue, from)
			continue
		}
		templates = append(templates, q.tx)
	}
	pool.Unlock()

	for _, tmpl := range templates {
		attempt := *tmpl // copy; Height is a value field, the shared slices stay read-only
		attempt.Height = next
		// admit + gossip. A not-yet-committed reveal (commit still pending) and
		// an already-landed reveal (commitment consumed) both fail here — the
		// former retries next tip, the latter is harmless until its window drops it
		if err := pool.PutNewTxFromLocal(&attempt); err != nil {
			log.Debugf("relay reveal %x not admissible at height %d: %v", attempt.GetHash(), next, err)
		}
	}
}

// OnTipChanged is the tip-movement hook the consensus wires into the chain: it
// deprecates the height-locked pool (Reset) and then, asynchronously, relays
// any held reveals at the new height. The relay runs in a goroutine so its db
// reads and gossip never block the block-import path, which fires this hook
// under the chain lock right after the write txn commits.
func (pool *TxPool) OnTipChanged() {
	pool.Reset()
	go pool.relayReveals()
}
