package ngp2p

import (
	"crypto/rand"
	"encoding/binary"
	mrand "math/rand"
	"sort"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// Dandelion++ two-phase propagation for locally-submitted txs and
// commitments. The commit-reveal mempool already hides tx CONTENT, but a
// first-spy adversary watching a few well-connected nodes still links the
// origin IP to each commitment, because local submissions flood instantly
// via pubsub. The router adds NETWORK-ORIGIN privacy:
//
//   - stem phase: the item hops peer-to-peer over the signed wired
//     protocol, one successor per epoch, so the eventual flood starts at
//     a node several unpredictable hops away from the true origin;
//   - fluff phase: each relay hop flips a q-coin and, with probability
//     DandelionFluffProb, broadcasts the item on the normal pubsub topic.
//
// Liveness is guaranteed twice over: a TTL fluffs at hop budget zero, and
// every stemmed item carries a local fail-safe timer that fluffs it if it
// has not been observed in the flood in time (black-holed stem path).
const (
	// DandelionEpoch is how long one stem successor stays pinned. Longer
	// epochs give an intersection attacker fewer routing graphs to
	// correlate; ~10 minutes follows the Dandelion++ paper (and Bitcoin
	// Core's implementation).
	DandelionEpoch = 10 * time.Minute

	// DandelionFluffProb (q) is the per-hop probability that a relayed
	// stem item transitions to fluff; the expected stem length is 1/q hops.
	DandelionFluffProb = 0.1

	// DandelionInitialTTL caps the stem length even when the q-coin keeps
	// missing or the successor graph loops back on itself.
	DandelionInitialTTL uint8 = 10

	// DandelionFailsafeMin/Max bound the random per-item fail-safe delay:
	// if a stemmed item is not seen fluffing before the timer fires, this
	// node fluffs it itself.
	DandelionFailsafeMin = 10 * time.Second
	DandelionFailsafeMax = 30 * time.Second

	// dandelionStempoolCap bounds the pending fail-safe map; past it new
	// stem items fluff immediately (liveness over privacy under flood).
	dandelionStempoolCap = 4096

	// dandelionSeenCap bounds the fluff-seen dedup set; entries are only
	// needed while a stem could still echo back (TTL hops + fail-safe
	// window), so a full reset on overflow is harmless.
	dandelionSeenCap = 1 << 16
)

// DandelionRouter implements the stem/fluff decision logic. All external
// effects (peer listing, stem sends, fluff broadcasts) and all entropy /
// time sources are injected, so the routing behavior is deterministic
// under test.
type DandelionRouter struct {
	// injected effects
	peers       func() []peer.ID
	stemTx      func(peer.ID, uint8, *ngtypes.FullTx) error
	stemCommit  func(peer.ID, uint8, *ngtypes.Commitment) error
	fluffTx     func(*ngtypes.FullTx) error
	fluffCommit func(*ngtypes.Commitment) error

	// injected entropy and clock (overridable in tests)
	Now       func() time.Time
	RandFloat func() float64 // uniform [0,1): drives the q-coin and the fail-safe jitter
	AfterFunc func(time.Duration, func()) *time.Timer

	// tunables (fixed defaults; tests and e2e shrink the timers)
	Epoch       time.Duration
	FluffProb   float64
	InitialTTL  uint8
	FailsafeMin time.Duration
	FailsafeMax time.Duration

	// secret salts the per-epoch successor choice so an adversary knowing
	// our peer list still cannot predict our stem route
	secret [32]byte

	mu     sync.Mutex
	seen   map[string]struct{}    // items observed in fluff (or fluffed by us)
	timers map[string]*time.Timer // the stempool: pending fail-safes, keyed like seen
	closed bool
}

// NewDandelionRouter builds a router around the given effects with the
// default Dandelion++ parameters and a fresh random routing secret.
func NewDandelionRouter(
	peers func() []peer.ID,
	stemTx func(peer.ID, uint8, *ngtypes.FullTx) error,
	stemCommit func(peer.ID, uint8, *ngtypes.Commitment) error,
	fluffTx func(*ngtypes.FullTx) error,
	fluffCommit func(*ngtypes.Commitment) error,
) *DandelionRouter {
	r := &DandelionRouter{
		peers:       peers,
		stemTx:      stemTx,
		stemCommit:  stemCommit,
		fluffTx:     fluffTx,
		fluffCommit: fluffCommit,

		Now:       time.Now,
		RandFloat: mrand.Float64,
		AfterFunc: time.AfterFunc,

		Epoch:       DandelionEpoch,
		FluffProb:   DandelionFluffProb,
		InitialTTL:  DandelionInitialTTL,
		FailsafeMin: DandelionFailsafeMin,
		FailsafeMax: DandelionFailsafeMax,

		seen:   make(map[string]struct{}),
		timers: make(map[string]*time.Timer),
	}

	if _, err := rand.Read(r.secret[:]); err != nil {
		panic(err) // no usable entropy: nothing sane to do
	}

	return r
}

// item keys: txs by their full hash, commitments by their unsigned hash —
// the same identities the broadcast validators report via OnTxSeen /
// OnCommitSeen.
func txKey(hash []byte) string     { return "t" + string(hash) }
func commitKey(hash []byte) string { return "c" + string(hash) }

// successor picks this epoch's stem relay: one peer chosen by
// Hash256(secret ‖ epochIndex) over the SORTED current peer list, so the
// choice is stable within an epoch (same peers -> same pick) yet
// unpredictable across nodes and epochs.
func (r *DandelionRouter) successor() (peer.ID, bool) {
	peers := r.peers()
	if len(peers) == 0 {
		return "", false
	}

	sort.Slice(peers, func(i, j int) bool { return peers[i] < peers[j] })

	buf := make([]byte, len(r.secret)+8)
	copy(buf, r.secret[:])
	epochIndex := uint64(r.Now().Unix()) / uint64(r.Epoch/time.Second)
	binary.LittleEndian.PutUint64(buf[len(r.secret):], epochIndex)

	digest := utils.Hash256(buf)
	idx := binary.LittleEndian.Uint64(digest[:8]) % uint64(len(peers))

	return peers[idx], true
}

// OriginateTx starts the stem phase for a locally-submitted tx. With no
// peers (or a failing stem send) it degrades to an immediate fluff, so a
// submission is never lost.
func (r *DandelionRouter) OriginateTx(tx *ngtypes.FullTx) error {
	return r.originate(txKey(tx.GetHash()),
		func(p peer.ID) error { return r.stemTx(p, r.InitialTTL, tx) },
		func() error { return r.fluffTx(tx) })
}

// OriginateCommit starts the stem phase for a locally-submitted
// commitment.
func (r *DandelionRouter) OriginateCommit(commit *ngtypes.Commitment) error {
	return r.originate(commitKey(commit.GetUnsignedHash()),
		func(p peer.ID) error { return r.stemCommit(p, r.InitialTTL, commit) },
		func() error { return r.fluffCommit(commit) })
}

// RelayTx handles a stem-phase tx received from a peer: with probability
// FluffProb (or at TTL zero) it fluffs, otherwise it forwards along OUR
// stem. Stem items are never added to the pool here — they enter pools
// only through the normal pubsub path once fluffed.
func (r *DandelionRouter) RelayTx(tx *ngtypes.FullTx, ttl uint8) {
	r.relay(txKey(tx.GetHash()), ttl,
		func(p peer.ID, nextTTL uint8) error { return r.stemTx(p, nextTTL, tx) },
		func() error { return r.fluffTx(tx) })
}

// RelayCommit handles a stem-phase commitment received from a peer.
func (r *DandelionRouter) RelayCommit(commit *ngtypes.Commitment, ttl uint8) {
	r.relay(commitKey(commit.GetUnsignedHash()), ttl,
		func(p peer.ID, nextTTL uint8) error { return r.stemCommit(p, nextTTL, commit) },
		func() error { return r.fluffCommit(commit) })
}

// SeenTx notifies the router that a tx surfaced in the fluff flood: its
// fail-safe (if any) is cancelled and later stem echoes are dropped.
func (r *DandelionRouter) SeenTx(txHash []byte) {
	r.markSeen(txKey(txHash))
}

// SeenCommit is SeenTx for commitments (keyed by the unsigned hash).
func (r *DandelionRouter) SeenCommit(commitUnsignedHash []byte) {
	r.markSeen(commitKey(commitUnsignedHash))
}

// Close stops every pending fail-safe timer.
func (r *DandelionRouter) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true
	for key, timer := range r.timers {
		timer.Stop()
		delete(r.timers, key)
	}
}

func (r *DandelionRouter) originate(key string, stem func(peer.ID) error, fluff func() error) error {
	succ, ok := r.successor()
	if !ok {
		// no peers to stem through: fluff directly, exactly as before
		// dandelion existed
		r.markSeen(key)
		return fluff()
	}

	if err := r.stemWithFailsafe(key, func() error { return stem(succ) }, fluff); err != nil {
		// the stem hop failed (peer gone, stream error): fall back to an
		// immediate fluff so the submission is never lost
		log.Debugf("dandelion stem to %s failed (%s), fluffing", succ, err)
		r.markSeen(key)
		return fluff()
	}

	return nil
}

func (r *DandelionRouter) relay(key string, ttl uint8, stem func(peer.ID, uint8) error, fluff func() error) {
	if r.isSeen(key) {
		return // already fluffed (or a stem echo of an item we handled)
	}

	if ttl == 0 || r.RandFloat() < r.FluffProb {
		r.fluffQuietly(key, fluff)
		return
	}

	succ, ok := r.successor()
	if !ok {
		r.fluffQuietly(key, fluff)
		return
	}

	if err := r.stemWithFailsafe(key, func() error { return stem(succ, ttl-1) }, fluff); err != nil {
		log.Debugf("dandelion stem relay to %s failed (%s), fluffing", succ, err)
		r.fluffQuietly(key, fluff)
	}
}

// stemWithFailsafe registers the fail-safe FIRST (so the item is covered
// even if the process is preempted mid-send), then performs the stem
// send. On send failure the fail-safe is dismissed again and the error
// returned for the caller's immediate-fluff fallback.
func (r *DandelionRouter) stemWithFailsafe(key string, stem func() error, fluff func() error) error {
	r.scheduleFailsafe(key, fluff)

	if err := stem(); err != nil {
		r.dismissFailsafe(key)
		return err
	}

	return nil
}

// scheduleFailsafe arms the liveness timer for a stemmed item: after a
// random delay in [FailsafeMin, FailsafeMax], if the item has not been
// seen fluffing, this node fluffs it itself. This keeps every submission
// live even if the whole stem path black-holes.
func (r *DandelionRouter) scheduleFailsafe(key string, fluff func() error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}
	if _, ok := r.seen[key]; ok {
		return
	}
	if _, ok := r.timers[key]; ok {
		return // already armed (stem echo)
	}
	if len(r.timers) >= dandelionStempoolCap {
		// stempool overflow: privacy yields to liveness — fluff now
		r.markSeenLocked(key)
		go func() { _ = fluff() }()
		return
	}

	delay := r.FailsafeMin + time.Duration(r.RandFloat()*float64(r.FailsafeMax-r.FailsafeMin))
	r.timers[key] = r.AfterFunc(delay, func() {
		r.mu.Lock()
		delete(r.timers, key)
		if r.closed {
			r.mu.Unlock()
			return
		}
		if _, ok := r.seen[key]; ok {
			r.mu.Unlock()
			return // fluffed elsewhere in time — nothing to do
		}
		r.markSeenLocked(key)
		r.mu.Unlock()

		log.Debugf("dandelion fail-safe fired, fluffing %x", key[1:])
		if err := fluff(); err != nil {
			log.Debugf("dandelion fail-safe fluff failed: %s", err)
		}
	})
}

func (r *DandelionRouter) dismissFailsafe(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if timer, ok := r.timers[key]; ok {
		timer.Stop()
		delete(r.timers, key)
	}
}

// fluffQuietly marks the item seen (cancelling any fail-safe) and
// broadcasts it, logging instead of propagating errors — relay handlers
// have no caller to report to.
func (r *DandelionRouter) fluffQuietly(key string, fluff func() error) {
	r.markSeen(key)
	if err := fluff(); err != nil {
		log.Debugf("dandelion fluff failed: %s", err)
	}
}

func (r *DandelionRouter) isSeen(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.seen[key]
	return ok
}

func (r *DandelionRouter) markSeen(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.markSeenLocked(key)

	if timer, ok := r.timers[key]; ok {
		timer.Stop()
		delete(r.timers, key)
	}
}

func (r *DandelionRouter) markSeenLocked(key string) {
	if len(r.seen) >= dandelionSeenCap {
		// entries only matter while a stem could still echo; reset wholesale
		r.seen = make(map[string]struct{})
	}
	r.seen[key] = struct{}{}
}
