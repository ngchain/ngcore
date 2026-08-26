package ngp2p

import (
	"math/big"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/ngchain/ngcore/ngtypes"
)

// routerHarness captures every effect a DandelionRouter can produce, with
// injected entropy and timers, so each decision path is asserted
// deterministically.
type routerHarness struct {
	router *DandelionRouter

	peerList []peer.ID

	stemTxCalls []struct {
		to  peer.ID
		ttl uint8
	}
	stemCommitCalls []struct {
		to  peer.ID
		ttl uint8
	}
	fluffTxCount     int
	fluffCommitCount int

	stemErr error

	// captured fail-safe callbacks, in scheduling order
	failsafes []func()
	delays    []time.Duration
}

func newRouterHarness(t *testing.T, peers ...peer.ID) *routerHarness {
	t.Helper()

	h := &routerHarness{peerList: peers}

	h.router = NewDandelionRouter(
		func() []peer.ID { return append([]peer.ID(nil), h.peerList...) },
		func(p peer.ID, ttl uint8, _ *ngtypes.FullTx) error {
			h.stemTxCalls = append(h.stemTxCalls, struct {
				to  peer.ID
				ttl uint8
			}{p, ttl})
			return h.stemErr
		},
		func(p peer.ID, ttl uint8, _ *ngtypes.Commitment) error {
			h.stemCommitCalls = append(h.stemCommitCalls, struct {
				to  peer.ID
				ttl uint8
			}{p, ttl})
			return h.stemErr
		},
		func(_ *ngtypes.FullTx) error { h.fluffTxCount++; return nil },
		func(_ *ngtypes.Commitment) error { h.fluffCommitCount++; return nil },
	)

	// deterministic entropy and clock
	h.router.RandFloat = func() float64 { return 0.99 } // default: never fluff by coin
	h.router.Now = func() time.Time { return time.Unix(0, 0) }
	h.router.AfterFunc = func(d time.Duration, f func()) *time.Timer {
		h.delays = append(h.delays, d)
		h.failsafes = append(h.failsafes, f)
		timer := time.NewTimer(time.Hour) // inert; only Stop() is exercised
		t.Cleanup(func() { timer.Stop() })
		return timer
	}

	return h
}

func testTx(t *testing.T) *ngtypes.FullTx {
	t.Helper()

	return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
		ngtypes.Address{}, big.NewInt(1), big.NewInt(0), nil, nil)
}

func testCommit(t *testing.T) *ngtypes.Commitment {
	t.Helper()

	return ngtypes.NewCommitment(ngtypes.ZERONET, 1, make([]byte, ngtypes.HashSize), big.NewInt(0))
}

// TestDandelionSuccessorDeterministic: same peers+secret+epoch always
// yield the same successor (regardless of the listing order), and the
// pick changes across epochs.
func TestDandelionSuccessorDeterministic(t *testing.T) {
	h := newRouterHarness(t, "peerA", "peerB", "peerC", "peerD")

	first, ok := h.router.successor()
	if !ok {
		t.Fatal("successor must exist with peers")
	}

	for i := 0; i < 10; i++ {
		if got, _ := h.router.successor(); got != first {
			t.Fatalf("successor changed within the epoch: %s != %s", got, first)
		}
	}

	// the pick must not depend on the raw listing order
	h.peerList = []peer.ID{"peerD", "peerB", "peerA", "peerC"}
	if got, _ := h.router.successor(); got != first {
		t.Fatalf("successor depends on the peer listing order: %s != %s", got, first)
	}

	// across epochs the successor re-rolls: with 4 peers and 64 epochs the
	// odds of never changing are (1/4)^63 — a change must appear
	changed := false
	for e := 1; e <= 64; e++ {
		epoch := e
		h.router.Now = func() time.Time { return time.Unix(int64(epoch)*int64(h.router.Epoch/time.Second), 0) }
		if got, _ := h.router.successor(); got != first {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("successor never re-rolled across 64 epochs")
	}

	// a different secret must (overwhelmingly) route differently — check
	// it at least yields a valid peer and the epoch-0 determinism holds
	h.router.Now = func() time.Time { return time.Unix(0, 0) }
	var other [32]byte
	other[0] = 0xff
	h.router.secret = other
	second, ok := h.router.successor()
	if !ok {
		t.Fatal("successor must exist with peers")
	}
	if got, _ := h.router.successor(); got != second {
		t.Fatal("successor not deterministic under the new secret")
	}
}

// TestDandelionZeroPeersFluffs: with no peers the originate path degrades
// to an immediate fluff and arms no fail-safe.
func TestDandelionZeroPeersFluffs(t *testing.T) {
	h := newRouterHarness(t) // no peers

	if err := h.router.OriginateTx(testTx(t)); err != nil {
		t.Fatal(err)
	}
	if err := h.router.OriginateCommit(testCommit(t)); err != nil {
		t.Fatal(err)
	}

	if h.fluffTxCount != 1 || h.fluffCommitCount != 1 {
		t.Fatalf("want immediate fluffs, got tx=%d commit=%d", h.fluffTxCount, h.fluffCommitCount)
	}
	if len(h.stemTxCalls)+len(h.stemCommitCalls) != 0 {
		t.Fatal("nothing must stem without peers")
	}
	if len(h.failsafes) != 0 {
		t.Fatal("an immediate fluff needs no fail-safe")
	}
}

// TestDandelionOriginateStems: with a peer, originate stems with the full
// TTL, does not fluff, and arms a fail-safe within [min, max].
func TestDandelionOriginateStems(t *testing.T) {
	h := newRouterHarness(t, "peerA")

	if err := h.router.OriginateTx(testTx(t)); err != nil {
		t.Fatal(err)
	}

	if len(h.stemTxCalls) != 1 {
		t.Fatalf("want 1 stem send, got %d", len(h.stemTxCalls))
	}
	if h.stemTxCalls[0].to != "peerA" || h.stemTxCalls[0].ttl != h.router.InitialTTL {
		t.Fatalf("stemmed to %s with ttl %d, want peerA with %d",
			h.stemTxCalls[0].to, h.stemTxCalls[0].ttl, h.router.InitialTTL)
	}
	if h.fluffTxCount != 0 {
		t.Fatal("originate must not fluff when the stem hop succeeds")
	}
	if len(h.failsafes) != 1 {
		t.Fatalf("want 1 fail-safe, got %d", len(h.failsafes))
	}
	if d := h.delays[0]; d < h.router.FailsafeMin || d > h.router.FailsafeMax {
		t.Fatalf("fail-safe delay %s outside [%s, %s]", d, h.router.FailsafeMin, h.router.FailsafeMax)
	}
}

// TestDandelionRelayForcedStem: with the q-coin forced to miss, a relayed
// item forwards along our stem with a decremented TTL.
func TestDandelionRelayForcedStem(t *testing.T) {
	h := newRouterHarness(t, "peerA")
	h.router.RandFloat = func() float64 { return 0.99 } // > q: never fluff

	h.router.RelayTx(testTx(t), 5)

	if len(h.stemTxCalls) != 1 || h.stemTxCalls[0].ttl != 4 {
		t.Fatalf("want 1 stem forward with ttl 4, got %+v", h.stemTxCalls)
	}
	if h.fluffTxCount != 0 {
		t.Fatal("forced-stem relay must not fluff")
	}
	if len(h.failsafes) != 1 {
		t.Fatal("a stemmed relay must arm the fail-safe")
	}
}

// TestDandelionRelayForcedFluff: with the q-coin forced to hit, a relayed
// item broadcasts instead of forwarding.
func TestDandelionRelayForcedFluff(t *testing.T) {
	h := newRouterHarness(t, "peerA")
	h.router.RandFloat = func() float64 { return 0.0 } // < q: always fluff

	h.router.RelayCommit(testCommit(t), 5)

	if h.fluffCommitCount != 1 {
		t.Fatalf("want 1 fluff, got %d", h.fluffCommitCount)
	}
	if len(h.stemCommitCalls) != 0 {
		t.Fatal("forced-fluff relay must not stem")
	}
}

// TestDandelionRelayTTLZeroFluffs: at hop budget zero the item always
// fluffs, whatever the coin says.
func TestDandelionRelayTTLZeroFluffs(t *testing.T) {
	h := newRouterHarness(t, "peerA")
	h.router.RandFloat = func() float64 { return 0.99 } // coin says stem

	h.router.RelayTx(testTx(t), 0)

	if h.fluffTxCount != 1 || len(h.stemTxCalls) != 0 {
		t.Fatalf("ttl 0 must force a fluff, got fluff=%d stem=%d", h.fluffTxCount, len(h.stemTxCalls))
	}
}

// TestDandelionRelayNoPeersFluffs: a relay hop with no successor fluffs.
func TestDandelionRelayNoPeersFluffs(t *testing.T) {
	h := newRouterHarness(t) // no peers

	h.router.RelayTx(testTx(t), 5)

	if h.fluffTxCount != 1 {
		t.Fatal("relay without peers must fluff")
	}
}

// TestDandelionStemFailureFallsBack: a failing stem send falls back to an
// immediate fluff and dismisses the fail-safe, so nothing is lost and
// nothing fires later.
func TestDandelionStemFailureFallsBack(t *testing.T) {
	h := newRouterHarness(t, "peerA")
	h.stemErr = errTest

	if err := h.router.OriginateTx(testTx(t)); err != nil {
		t.Fatal(err)
	}

	if h.fluffTxCount != 1 {
		t.Fatal("a failed stem hop must fluff immediately")
	}
	// the pre-armed fail-safe was dismissed: firing it must not re-fluff
	for _, fire := range h.failsafes {
		fire()
	}
	if h.fluffTxCount != 1 {
		t.Fatal("a dismissed fail-safe must not fluff again")
	}
}

// TestDandelionFailsafeFluffsUnseen: the fail-safe timer fluffs an item
// that never surfaced in the flood — and only once.
func TestDandelionFailsafeFluffsUnseen(t *testing.T) {
	h := newRouterHarness(t, "peerA")

	if err := h.router.OriginateTx(testTx(t)); err != nil {
		t.Fatal(err)
	}
	if len(h.failsafes) != 1 {
		t.Fatal("expected an armed fail-safe")
	}

	h.failsafes[0]() // the stem black-holed: the timer expires
	if h.fluffTxCount != 1 {
		t.Fatalf("fail-safe must fluff the unseen item, got %d", h.fluffTxCount)
	}

	h.failsafes[0]() // double fire must be idempotent
	if h.fluffTxCount != 1 {
		t.Fatal("fail-safe must fluff only once")
	}
}

// TestDandelionFailsafeCancelledBySeen: an item observed in the fluff
// flood cancels its fail-safe — the timer firing later is a no-op.
func TestDandelionFailsafeCancelledBySeen(t *testing.T) {
	h := newRouterHarness(t, "peerA")
	tx := testTx(t)

	if err := h.router.OriginateTx(tx); err != nil {
		t.Fatal(err)
	}

	h.router.SeenTx(tx.GetHash()) // the pubsub validator saw it fluff

	h.failsafes[0]()
	if h.fluffTxCount != 0 {
		t.Fatal("a seen item must not be re-fluffed by the fail-safe")
	}
}

// TestDandelionRelaySeenDropped: a stem echo of an already-fluffed item
// is dropped entirely (no forward, no fluff).
func TestDandelionRelaySeenDropped(t *testing.T) {
	h := newRouterHarness(t, "peerA")
	tx := testTx(t)

	h.router.SeenTx(tx.GetHash())
	h.router.RelayTx(tx, 5)

	if h.fluffTxCount != 0 || len(h.stemTxCalls) != 0 {
		t.Fatal("a seen item must be dropped on stem receipt")
	}

	commit := testCommit(t)
	h.router.SeenCommit(commit.GetUnsignedHash())
	h.router.RelayCommit(commit, 5)

	if h.fluffCommitCount != 0 || len(h.stemCommitCalls) != 0 {
		t.Fatal("a seen commitment must be dropped on stem receipt")
	}
}

// TestDandelionCloseStopsFailsafes: Close dismisses pending timers; a
// late fire is a no-op.
func TestDandelionCloseStopsFailsafes(t *testing.T) {
	h := newRouterHarness(t, "peerA")

	if err := h.router.OriginateTx(testTx(t)); err != nil {
		t.Fatal(err)
	}

	h.router.Close()

	h.failsafes[0]()
	if h.fluffTxCount != 0 {
		t.Fatal("a fail-safe must not fire after Close")
	}
}

var errTest = errTestType{}

type errTestType struct{}

func (errTestType) Error() string { return "test stem error" }
