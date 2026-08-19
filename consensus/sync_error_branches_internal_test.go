package consensus

// Error-path coverage for converging and snapshot converging: a fakePeer
// serves a detached / out-of-range branch so the atomic apply
// (SwitchToBranch / ApplySnapshot) fails after the download succeeds.

import (
	"bytes"
	"testing"

	"github.com/ngchain/ngcore/ngp2p/wired"
	"github.com/ngchain/ngcore/ngtypes"
)

// detachedBlock seals a valid-PoW block@1 that hangs off an unknown parent,
// so any chain starting with it fails to attach.
func detachedBlock(t *testing.T) *ngtypes.FullBlock {
	t.Helper()

	key, _ := ngtypes.GenerateKey()
	b := ngtypes.NewBareBlock(
		ngtypes.ZERONET,
		1,
		uint64(0x1000_0000), // fixed past timestamp, deterministic
		bytes.Repeat([]byte{0x7c}, 32),
		ngtypes.GetGenesisBlock(ngtypes.ZERONET).GetActualDiff(),
	)
	if err := b.ToUnsealing([]*ngtypes.FullTx{CreateGenerateTx(ngtypes.ZERONET, key, 1, nil)}); err != nil {
		t.Fatal(err)
	}
	return sealBlock(t, b)
}

// chainReply builds a fakePeer reply serving the given blocks as a ChainMsg.
func chainReply(t *testing.T, blocks ...*ngtypes.FullBlock) reply {
	return func(fp *fakePeer, reqID []byte) *wired.Message {
		return &wired.Message{
			Header:  fp.header(reqID, wired.ChainMsg),
			Payload: mustRLP(t, &wired.ChainPayload{Blocks: blocks}),
		}
	}
}

// TestDoConvergingApplyFails: the download returns a detached branch, so
// SwitchToBranch refuses it (covers doConverging's apply-error return).
func TestDoConvergingApplyFails(t *testing.T) {
	fp := newFakePeer(t, ngtypes.ZERONET, chainReply(t, detachedBlock(t)))

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	localKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, local, localKey, 1) // a local fork so getBlocksForConverging has a hash to send
	connectFake(t, local, fp)

	rec := recordForFake(fp, 0, 12, make([]byte, 32), []byte{0x01})
	if err := local.SyncMod.doConverging(rec); err == nil {
		t.Fatal("doConverging onto a detached branch must fail")
	}
}

// TestDoSnapshotConvergingCutAboveCheckpoint: the fetched fork branch's
// first block already sits above the remote checkpoint, so the cut guard
// rejects it (covers the tipHeight>checkpoint / cut<=0 branch).
func TestDoSnapshotConvergingCutAboveCheckpoint(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	remoteKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, remoteKey, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true, SnapshotMode: true})
	localKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, local, localKey, 12)

	connectNodes(t, local, remote)

	// craft a record whose checkpoint sits BELOW the fork's first block:
	// getBlocksForConverging returns local blocks 1..12 (branch[0]@1),
	// checkpoint forced to 0 -> branch[0].height(1) > checkpoint(0) and the
	// cut lands <= 0
	rec := NewRemoteRecord(remote.LocalNode.ID(), 0, 12,
		remote.Chain.GetLatestCheckpoint().GetHash(), []byte{0x01})
	rec.checkpointHeight = 0

	if err := local.SyncMod.doSnapshotConverging(rec); err == nil {
		t.Fatal("doSnapshotConverging with the fork above the checkpoint must fail")
	}
}

// TestDoSnapshotSyncSheetFails: the block range downloads fine but the
// checkpoint state sheet is unservable (wrong hash), so the sheet fetch
// fails after the range fetch.
func TestDoSnapshotSyncSheetFails(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, key, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true, SnapshotMode: true})
	connectNodes(t, local, remote)

	// checkpointHeight 10 (servable range) but a hash the remote cannot map
	// to a snapshot -> getRemoteStateSheet rejected
	rec := NewRemoteRecord(remote.LocalNode.ID(), 0, 12,
		bytes.Repeat([]byte{0xcd}, 32), []byte{0x01})
	if err := local.SyncMod.doSnapshotSync(rec); err == nil {
		t.Fatal("doSnapshotSync with an unservable sheet must fail")
	}
	// the range fetch happened but nothing was applied
	if local.Chain.GetLatestBlockHeight() != 0 {
		t.Fatal("a failed snapshot sync must not move the chain")
	}
}

// TestDoSnapshotConvergingBranchFails: the local chain is at genesis, so
// getBlocksForConverging cannot find a diffpoint and errors before any
// apply.
func TestDoSnapshotConvergingBranchFails(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, key, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true, SnapshotMode: true})
	connectNodes(t, local, remote)

	// origin-only local: nothing to compare, getBlocksForConverging errors
	if err := local.SyncMod.doSnapshotConverging(recordFor(remote)); err == nil {
		t.Fatal("doSnapshotConverging on an origin-only chain must fail")
	}
}

// TestDoSnapshotConvergingSheetFails: the branch is fetched, trimmed to the
// checkpoint, but the state sheet request is rejected (covers the sheet
// error return after the branch is assembled).
func TestDoSnapshotConvergingSheetFails(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	remoteKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, remoteKey, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true, SnapshotMode: true})
	localKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, local, localKey, 12)

	connectNodes(t, local, remote)

	// checkpoint hash the remote cannot serve as a snapshot: the branch
	// assembles (trimmed to 10) but getRemoteStateSheet is rejected
	rec := NewRemoteRecord(remote.LocalNode.ID(), 0, 12,
		bytes.Repeat([]byte{0xab}, 32), []byte{0x01})

	if err := local.SyncMod.doSnapshotConverging(rec); err == nil {
		t.Fatal("doSnapshotConverging with an unservable sheet must fail")
	}
}
