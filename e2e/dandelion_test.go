package e2e

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestDandelionStemLiveness drives a commitment and its reveal tx through
// the Dandelion++ stem/fluff path on a 3-node line A-B-C (A and C are not
// directly connected): a local submission on A leaves over a single wired
// stem stream instead of flooding, hops peer-to-peer, and eventually
// fluffs via pubsub — the items must land in C's pool. The per-node
// fail-safe timers are shrunk so a black-holed stem hop re-fluffs within
// the test budget; every path (q-coin fluff, TTL exhaustion, fail-safe)
// converges well inside the waits below.
func TestDandelionStemLiveness(t *testing.T) {
	nodeA := newNode(t)
	nodeB := newNode(t)
	nodeC := newNode(t)
	connect(t, nodeA, nodeB)
	connect(t, nodeB, nodeC)

	// dandelion is ON by default; shrink only the fail-safe window
	for _, n := range []*testNode{nodeA, nodeB, nodeC} {
		if n.local.Dandelion == nil {
			t.Fatal("dandelion must be enabled by default")
		}
		n.local.Dandelion.FailsafeMin = 500 * time.Millisecond
		n.local.Dandelion.FailsafeMax = time.Second
	}

	key, _ := ngtypes.GenerateKey()

	// fund the key on A; the whole line must converge
	b1 := mineAndSubmit(t, nodeA, key)
	waitTip(t, nodeC, b1.GetHash(), 10*time.Second)
	b2 := mineAndSubmit(t, nodeA, key)
	waitTip(t, nodeC, b2.GetHash(), 10*time.Second)

	// the reveal (locked to height 4) and its blind commitment (height 3)
	var dest ngtypes.Address
	dest[0] = 0xdd
	tx := revealTx(t, ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 4,
		dest, big.NewInt(10), big.NewInt(1), nil, nil), key)
	commit := commitFor(t, tx, key, 3)

	// submit the commitment locally on A: it stems away instead of
	// flooding, and must still surface in C's pool (liveness)
	if err := nodeA.pow.Pool.PutNewCommitmentFromLocal(commit); err != nil {
		t.Fatalf("submit commitment on nodeA: %v", err)
	}

	waitFor(t, 20*time.Second, "commitment never reached nodeC's pool", func() bool {
		for _, c := range nodeC.pow.Pool.ListCommitments() {
			if bytes.Equal(c.Hash, commit.Hash) {
				return true
			}
		}
		return false
	})

	// land the commitment at height 3 so the reveal becomes admissible
	b3 := mineOnAll(t, b2, key, []*ngtypes.Commitment{commit})
	if err := nodeA.pow.MinedNewBlock(b3); err != nil {
		t.Fatalf("submit commit block on nodeA: %v", err)
	}
	waitTip(t, nodeB, b3.GetHash(), 10*time.Second)
	waitTip(t, nodeC, b3.GetHash(), 10*time.Second)

	// submit the reveal locally on A: same stem path, same liveness bar
	if err := nodeA.pow.Pool.PutNewTxFromLocal(tx); err != nil {
		t.Fatalf("submit reveal on nodeA: %v", err)
	}

	waitFor(t, 20*time.Second, "reveal never reached nodeC's pool", func() bool {
		exists, _ := nodeC.pow.Pool.IsInPool(tx.GetHash())
		return exists
	})
}

// waitFor polls cond until it holds or the timeout expires.
func waitFor(t *testing.T, timeout time.Duration, msg string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatal(msg)
}
