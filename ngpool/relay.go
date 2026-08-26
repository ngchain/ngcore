package ngpool

import (
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
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

// queuedCommit is a signed commitment the node relays until it lands on chain.
type queuedCommit struct {
	commit   *ngtypes.Commitment
	expireAt uint64
}

// relayExpiry is how many block heights a relayed half is retried before it is
// dropped: two commit windows, enough to cover a commitment that misses its
// first target block and then its reveal window opening at the later height.
func relayExpiry(next uint64) uint64 { return next + 2*ngtypes.CommitWindow }

// RelayCommit accepts a signed commitment to relay on the sender's behalf: the
// node re-submits it (retargeting its height, no re-signing) on every tip until
// it is recorded on chain, then stops — a commitment is single-inclusion, so it
// is never double-charged. Paired with RelayReveal this makes the WHOLE
// commit-reveal flow fire-and-forget: the wallet signs both halves once and may
// go offline.
func (pool *TxPool) RelayCommit(commit *ngtypes.Commitment) error {
	if !commit.IsSigned() {
		return ngtypes.ErrCommitUnsigned
	}
	from, err := commit.From()
	if err != nil {
		return err
	}

	next := pool.chain.GetLatestBlockHeight() + 1

	// validate against the next block (signature, affordability, not already a
	// pending duplicate); a probe height satisfies CheckError's height match
	probe := *commit
	probe.Height = next
	if err := pool.db.View(func(txn *bbolt.Tx) error {
		return ngstate.CheckCommitment(txn, &probe, next)
	}); err != nil {
		return errors.Wrap(err, "commitment is not admissible")
	}

	pool.Lock()
	if _, exists := pool.commitQueue[from]; !exists && len(pool.commitQueue) >= pool.MaxSize {
		pool.Unlock()
		return ErrPoolFull
	}
	pool.commitQueue[from] = &queuedCommit{commit: commit, expireAt: relayExpiry(next)}
	pool.Unlock()

	pool.relayCommits()
	return nil
}

// relayCommits re-submits every queued commitment that has not yet landed,
// retargeted to the current next height, and drops the ones whose window has
// closed. A commitment already recorded on chain is skipped (never re-included,
// so never double-charged) but kept queued so a reorg that orphans it re-lands
// it. Safe to run from a goroutine: it never holds the pool lock across
// PutCommitment and works on a copy of each commitment (only Height differs).
func (pool *TxPool) relayCommits() {
	next := pool.chain.GetLatestBlockHeight() + 1

	type pendingCommit struct {
		from   ngtypes.Address
		commit *ngtypes.Commitment
	}
	pool.Lock()
	templates := make([]pendingCommit, 0, len(pool.commitQueue))
	for from, q := range pool.commitQueue {
		if next > q.expireAt {
			delete(pool.commitQueue, from)
			continue
		}
		templates = append(templates, pendingCommit{from: from, commit: q.commit})
	}
	pool.Unlock()

	for _, pc := range templates {
		tmpl := pc.commit
		landed := false
		_ = pool.db.View(func(txn *bbolt.Tx) error {
			landed = ngstate.CommitOnChain(txn, pc.from, tmpl.Hash, next)
			return nil
		})
		if landed {
			continue // recorded on chain — do not re-include (single-inclusion)
		}
		attempt := *tmpl // copy; only Height differs, the shared slices stay read-only
		attempt.Height = next
		if err := pool.PutNewCommitmentFromLocal(&attempt); err != nil {
			log.Debugf("relay commit %x not admissible at height %d: %v", attempt.Hash, next, err)
		}
	}
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

	// gate the reveal up front (signature, content rules, and that From can
	// afford it) so a forged or unfunded reveal cannot squat a queue slot until
	// its window lapses — the commit-reveal check is intentionally skipped,
	// since the commitment need not be on chain yet. An effect tx signs its
	// height-independent SigningHash, so a probe height sidesteps Verify's
	// genesis (height 0) short-circuit without changing the digest; the shared
	// reveal is untouched.
	probe := *reveal
	if probe.Height == 0 {
		probe.Height = 1
	}
	if err := pool.db.View(func(txn *bbolt.Tx) error {
		return ngstate.CheckRevealExceptCommitment(txn, &probe)
	}); err != nil {
		return errors.Wrap(err, "reveal is not admissible")
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
	// keep the reveal long enough to outlast a commitment that lands a block or
	// two late and then its own reveal window opening at that later height
	pool.revealQueue[from] = &queuedReveal{tx: reveal, expireAt: relayExpiry(next)}
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
// both held commitments and reveals at the new height (commitments first, so a
// freshly-landed commit is revealable the same round). The relay runs in a
// goroutine so its db reads and gossip never block the block-import path, which
// fires this hook under the chain lock right after the write txn commits.
func (pool *TxPool) OnTipChanged() {
	pool.Reset()
	go func() {
		pool.relayCommits()
		pool.relayReveals()
	}()
}
