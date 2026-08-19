package consensus

// Coverage for the bootstrap and background loop machinery: the many
// sync/converge/snapshot sub-branches only fire with the right mix of
// remotes (multiple peers to drive the sort comparators, a fork to force
// converging, snapshot mode for the snapshot paths).

import (
	"bytes"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestBootstrapSnapshotMode: a snapshot-mode node bootstraps against a
// remote ahead, exercising the doSnapshotSync branch of the sync loop.
func TestBootstrapSnapshotMode(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	remoteChain := mineBlocks(t, remote, key, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true, SnapshotMode: true})
	connectNodes(t, local, remote)

	local.SyncMod.bootstrap()

	// snapshot sync fast-forwards to the checkpoint@10
	if h := local.Chain.GetLatestBlockHeight(); h != 10 {
		t.Fatalf("height after snapshot bootstrap = %d, want 10", h)
	}
	if !bytes.Equal(local.Chain.GetLatestBlockHash(), remoteChain[9].GetHash()) {
		t.Fatal("local tip is not the remote checkpoint block")
	}
}

// TestBootstrapConverges: a forked local node bootstraps against a heavier
// remote, so the plain sync fails and the converge loop takes over.
func TestBootstrapConverges(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	remoteKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, remoteKey, 12) // heavier: checkpoint@10 vs local genesis

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	localKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, local, localKey, 1) // a fork the remote does not know

	connectNodes(t, local, remote)

	local.SyncMod.bootstrap()

	// converging rewrote the fork prefix, so the local tip now descends
	// from the remote's chain at the diffpoint height
	h := local.Chain.GetLatestBlockHeight()
	if h < 1 {
		t.Fatalf("height after converge bootstrap = %d, want >=1", h)
	}
	remoteAtLocalTip, err := remote.Chain.GetBlockByHeight(h)
	if err != nil {
		t.Fatalf("remote lacks a block at the local tip height: %s", err)
	}
	if !bytes.Equal(local.Chain.GetLatestBlockHash(), remoteAtLocalTip.GetHash()) {
		t.Fatal("the local tip did not converge onto the remote chain")
	}
}

// TestBootstrapMultipleRemotes wires the local node to two remotes so the
// bootstrap sort comparators (lastChatTime, latest) actually run.
func TestBootstrapMultipleRemotes(t *testing.T) {
	remoteA := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	keyA, _ := ngtypes.GenerateKey()
	mineBlocks(t, remoteA, keyA, 12)

	remoteB := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	keyB, _ := ngtypes.GenerateKey()
	mineBlocks(t, remoteB, keyB, 6)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	connectNodes(t, local, remoteA)
	connectNodes(t, local, remoteB)

	local.SyncMod.bootstrap()

	if len(local.SyncMod.store) != 2 {
		t.Fatalf("bootstrap recorded %d remotes, want 2", len(local.SyncMod.store))
	}
	// the taller remote (A) wins the sync
	if h := local.Chain.GetLatestBlockHeight(); h != 12 {
		t.Fatalf("height after multi-remote bootstrap = %d, want 12", h)
	}
}
