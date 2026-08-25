package broadcast

import (
	"testing"
	"time"

	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	"github.com/ngchain/ngcore/ngtypes"
)

// NOTE: the panic branches in NewBroadcastProtocol (broadcast.go:56-88) guard
// pubsub setup calls — NewFloodSub, RegisterTopicValidator, Join, Subscribe —
// each of which is given a fresh pubsub instance and freshly-derived topic
// names on a healthy mocknet host. None can be made to fail from outside the
// package without breaking libp2p internals, so those defensive panics stay
// uncovered (see the coverage report).

// TestListenerContinuesOnSubError drives the sub.Next error branch of the
// listeners while the context is still alive: cancelling a subscription makes
// the next sub.Next return an error, the listener logs it and continues (it
// does not return, since b.ctx is not cancelled). The goroutine then spins on
// the cancelled sub; we simply confirm the listeners never touch the channels
// and the protocol stays healthy enough to close cleanly.
func TestListenerContinuesOnSubError(t *testing.T) {
	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	h, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	b := NewBroadcastProtocol(h, ngtypes.ZERONET,
		make(chan *ngtypes.FullBlock, 1), make(chan *ngtypes.FullTx, 1), make(chan *ngtypes.Commitment, 1))

	// start the listeners, then cancel the subscriptions WITHOUT cancelling
	// b.ctx: sub.Next returns an error and the loop hits the log+continue path
	b.GoServe()

	b.subscriptions[b.blockTopic].Cancel()
	b.subscriptions[b.txTopic].Cancel()

	// let the listeners observe the error and pass through the log+continue
	// branch at least once, then stop them promptly so the loop does not spin
	time.Sleep(20 * time.Millisecond)

	// no spurious deliveries
	select {
	case blk := <-b.OnBlock:
		t.Errorf("unexpected block delivery: %v", blk)
	case tx := <-b.OnTx:
		t.Errorf("unexpected tx delivery: %v", tx)
	default:
	}

	// cancelling the context lets the loops take their shutdown return path
	b.cancel()
	time.Sleep(50 * time.Millisecond)
}
