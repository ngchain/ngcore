package consensus

// Small error-path coverage for the PoWork lifecycle: a double Stop
// surfaces the p2p close error, a broadcast on a closed node fails
// MinedNewBlock's broadcast step, and the event loop's block-import failure
// branch is driven with a detached broadcast block.

import (
	"testing"
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestStopTwiceLogsCloseError: the second Stop closes an already-closed p2p
// node, hitting the close-error branch (the cleanup Stop then no-ops).
func TestStopTwiceLogsCloseError(t *testing.T) {
	pow := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})

	pow.Stop()
	// a second Stop re-closes the host; libp2p returns an error, exercising
	// the error branch. It must not panic.
	pow.Stop()
}

// TestMinedNewBlockBroadcastFails: after the p2p node is closed, applying a
// mined block still succeeds but broadcasting it fails.
func TestMinedNewBlockBroadcastFails(t *testing.T) {
	pow := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()

	block := sealBlock(t, pow.GetBlockTemplate(key).(*ngtypes.FullBlock))

	// close the p2p node so BroadcastBlock has no live pubsub topic
	if err := pow.LocalNode.Close(); err != nil {
		t.Fatalf("failed to close the p2p node: %s", err)
	}

	err := pow.MinedNewBlock(block)
	if err == nil {
		t.Fatal("MinedNewBlock must fail when the broadcast cannot go out")
	}
	// the block was still applied before the broadcast attempt
	if pow.Chain.GetLatestBlockHeight() != 1 {
		t.Fatalf("height = %d, want the block applied before the broadcast failure", pow.Chain.GetLatestBlockHeight())
	}
}

// TestEventLoopImportFailure feeds a detached block into the broadcast
// channel: the import fails and only logs (the failure branch of the block
// consumer).
func TestEventLoopImportFailure(t *testing.T) {
	pow := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})

	pow.GoLoop()

	// a block whose parent is unknown cannot be applied; it is parked as an
	// orphan or dropped, but the tip must not move
	bad := detachedBlock(t)
	pow.LocalNode.OnBlock <- bad

	time.Sleep(300 * time.Millisecond)
	if pow.Chain.GetLatestBlockHeight() != 0 {
		t.Fatal("a detached broadcast block must not move the tip")
	}
}
