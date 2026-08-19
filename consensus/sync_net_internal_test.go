package consensus

// two-node sync tests: a "remote" full node mines a real zeronet chain
// on loopback and the "local" node exercises every sync path (status
// pings, chain download, converging, checkpoint fast-sync and the
// snapshot mode) against it.

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/ngchain/ngcore/ngtypes"
)

const bogusPeer = peer.ID("bogus-peer-id")

func TestInitPoWConsensusWithBootstrapEnabled(t *testing.T) {
	// bootstrapping without any known peer just logs and returns
	pow := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: false})

	if pow.Chain.GetLatestBlockHeight() != 0 {
		t.Fatal("a peerless bootstrap must leave the chain untouched")
	}
}

func TestGetRemoteStatus(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, key, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	connectNodes(t, local, remote)
	mod := local.SyncMod

	// the first ping creates the record...
	if err := mod.getRemoteStatus(remote.LocalNode.ID()); err != nil {
		t.Fatalf("getRemoteStatus failed: %s", err)
	}
	rec := mod.store[remote.LocalNode.ID()]
	if rec == nil {
		t.Fatal("the pong did not create a remote record")
	}
	if rec.latest != 12 {
		t.Fatalf("remote latest = %d, want 12", rec.latest)
	}
	if rec.checkpointHeight != 10 {
		t.Fatalf("remote checkpoint height = %d, want 10", rec.checkpointHeight)
	}
	if !bytes.Equal(rec.checkpointHash, remote.Chain.GetLatestCheckpoint().GetHash()) {
		t.Fatal("remote checkpoint hash does not match the remote chain")
	}

	// ...the second one updates it in place
	mineBlocks(t, remote, key, 1)
	if err := mod.getRemoteStatus(remote.LocalNode.ID()); err != nil {
		t.Fatalf("second getRemoteStatus failed: %s", err)
	}
	if len(mod.store) != 1 {
		t.Fatalf("store holds %d records, want 1", len(mod.store))
	}
	if mod.store[remote.LocalNode.ID()].latest != 13 {
		t.Fatalf("record latest after the update = %d, want 13", mod.store[remote.LocalNode.ID()].latest)
	}

	// an unreachable peer only logs (no error, no record)
	if err := mod.getRemoteStatus(bogusPeer); err != nil {
		t.Fatalf("getRemoteStatus to an unreachable peer = %v, want nil", err)
	}
	if len(mod.store) != 1 {
		t.Fatal("an unreachable peer must not be recorded")
	}
}

func TestDoSync(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, key, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	connectNodes(t, local, remote)
	mod := local.SyncMod
	rec := recordFor(remote)

	// a busy locker skips the sync silently
	mod.Locker.Lock()
	if err := mod.doSync(rec); err != nil {
		t.Fatalf("doSync under an active locker = %v, want nil", err)
	}
	mod.Locker.Unlock()
	if local.Chain.GetLatestBlockHeight() != 0 {
		t.Fatal("a skipped sync must not move the chain")
	}

	// the real sync downloads and applies the whole remote chain
	if err := mod.doSync(rec); err != nil {
		t.Fatalf("doSync failed: %s", err)
	}
	if h := local.Chain.GetLatestBlockHeight(); h != 12 {
		t.Fatalf("height after doSync = %d, want 12", h)
	}
	if !bytes.Equal(local.Chain.GetLatestBlockHash(), remote.Chain.GetLatestBlockHash()) {
		t.Fatal("local tip does not match the remote tip after doSync")
	}

	// an unreachable remote fails the sync
	badRec := NewRemoteRecord(bogusPeer, 0, 20, nil, nil)
	if err := mod.doSync(badRec); err == nil {
		t.Fatal("doSync against an unreachable remote must fail")
	}
}

func TestDoSyncRejectedOnFork(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	remoteKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, remoteKey, 3)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	localKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, local, localKey, 1) // fork: the remote does not know this tip

	connectNodes(t, local, remote)

	err := local.SyncMod.doSync(recordFor(remote))
	if !errors.Is(err, ErrMsgRejected) {
		t.Fatalf("doSync from a forked tip = %v, want ErrMsgRejected", err)
	}
}

func TestDoConverging(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	remoteKey, _ := ngtypes.GenerateKey()
	remoteChain := mineBlocks(t, remote, remoteKey, 3)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	localKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, local, localKey, 1) // the fork to abandon

	connectNodes(t, local, remote)
	mod := local.SyncMod
	rec := recordFor(remote)

	// a busy locker skips the converge silently
	mod.Locker.Lock()
	if err := mod.doConverging(rec); err != nil {
		t.Fatalf("doConverging under an active locker = %v, want nil", err)
	}
	mod.Locker.Unlock()

	if err := mod.doConverging(rec); err != nil {
		t.Fatalf("doConverging failed: %s", err)
	}

	// converging rewrites the diverged prefix; the remainder arrives
	// through the normal sync
	if h := local.Chain.GetLatestBlockHeight(); h != 1 {
		t.Fatalf("height after converging = %d, want 1", h)
	}
	if !bytes.Equal(local.Chain.GetLatestBlockHash(), remoteChain[0].GetHash()) {
		t.Fatal("the local fork block was not replaced by the remote block@1")
	}

	if err := mod.doSync(rec); err != nil {
		t.Fatalf("doSync after converging failed: %s", err)
	}
	if h := local.Chain.GetLatestBlockHeight(); h != 3 {
		t.Fatalf("height after converge+sync = %d, want 3", h)
	}
}

func TestDoConvergingFromGenesisFails(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, key, 3)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	connectNodes(t, local, remote)

	// with the local chain still at its origin there is nothing to
	// compare, so converging must refuse
	if err := local.SyncMod.doConverging(recordFor(remote)); err == nil {
		t.Fatal("doConverging on an origin-only chain must fail")
	}
}

func TestSwitchToRemoteCheckpoint(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	remoteChain := mineBlocks(t, remote, key, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	connectNodes(t, local, remote)
	mod := local.SyncMod

	// a busy locker skips silently
	mod.Locker.Lock()
	if err := mod.switchToRemoteCheckpoint(recordFor(remote)); err != nil {
		t.Fatalf("switchToRemoteCheckpoint under an active locker = %v, want nil", err)
	}
	mod.Locker.Unlock()

	// a young remote (checkpoint within 2 rounds) resolves to genesis:
	// nothing moves
	if err := mod.switchToRemoteCheckpoint(recordFor(remote)); err != nil {
		t.Fatalf("switchToRemoteCheckpoint (genesis case) failed: %s", err)
	}
	if local.Chain.GetLatestBlockHeight() != 0 {
		t.Fatal("the genesis checkpoint must keep the chain at height 0")
	}

	// a remote reporting latest=30 puts the safe checkpoint at
	// 30-2*round=10, which the remote serves
	cp := remote.Chain.GetLatestCheckpoint()
	farRec := NewRemoteRecord(remote.LocalNode.ID(), 0, 30, cp.GetHash(), cp.GetActualDiff().Bytes())
	if err := mod.switchToRemoteCheckpoint(farRec); err != nil {
		t.Fatalf("switchToRemoteCheckpoint (fetch case) failed: %s", err)
	}
	if h := local.Chain.GetLatestBlockHeight(); h != 10 {
		t.Fatalf("height after the checkpoint switch = %d, want 10", h)
	}
	if o := local.Chain.GetOriginBlock(); o.GetHeight() != 10 ||
		!bytes.Equal(o.GetHash(), remoteChain[9].GetHash()) {
		t.Fatal("the origin must become the remote block@10")
	}

	// a checkpoint the remote cannot serve gets rejected
	tooFarRec := NewRemoteRecord(remote.LocalNode.ID(), 0, 50, cp.GetHash(), cp.GetActualDiff().Bytes())
	if err := mod.switchToRemoteCheckpoint(tooFarRec); !errors.Is(err, ErrMsgRejected) {
		t.Fatalf("switchToRemoteCheckpoint beyond the remote = %v, want ErrMsgRejected", err)
	}

	// an unreachable remote fails
	deadRec := NewRemoteRecord(bogusPeer, 0, 30, cp.GetHash(), cp.GetActualDiff().Bytes())
	if err := mod.switchToRemoteCheckpoint(deadRec); err == nil {
		t.Fatal("switchToRemoteCheckpoint to an unreachable peer must fail")
	}
}

func TestGetRemoteChainUnreachable(t *testing.T) {
	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})

	if _, err := local.SyncMod.getRemoteChain(bogusPeer, [][]byte{}, nil); err == nil {
		t.Fatal("getRemoteChain to an unreachable peer must fail")
	}
}

func TestGetRemoteStateSheet(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, key, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	connectNodes(t, local, remote)
	mod := local.SyncMod

	// the checkpoint snapshot is servable
	rec := recordFor(remote)
	sheet, err := mod.getRemoteStateSheet(rec)
	if err != nil {
		t.Fatalf("getRemoteStateSheet failed: %s", err)
	}
	if sheet.Height != 10 {
		t.Fatalf("sheet height = %d, want the checkpoint 10", sheet.Height)
	}
	if !bytes.Equal(sheet.BlockHash, remote.Chain.GetLatestCheckpoint().GetHash()) {
		t.Fatal("the sheet does not bind the remote checkpoint")
	}

	// an unknown snapshot hash is rejected
	badRec := NewRemoteRecord(remote.LocalNode.ID(), 0, 12, make([]byte, 32), []byte{0x01})
	if _, err := mod.getRemoteStateSheet(badRec); !errors.Is(err, ErrMsgRejected) {
		t.Fatalf("getRemoteStateSheet with a wrong hash = %v, want ErrMsgRejected", err)
	}

	// an unreachable remote fails
	deadRec := NewRemoteRecord(bogusPeer, 0, 12, make([]byte, 32), []byte{0x01})
	if _, err := mod.getRemoteStateSheet(deadRec); err == nil {
		t.Fatal("getRemoteStateSheet to an unreachable peer must fail")
	}
}

func TestDoSnapshotSync(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	remoteChain := mineBlocks(t, remote, key, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true, SnapshotMode: true})
	connectNodes(t, local, remote)
	mod := local.SyncMod
	rec := recordFor(remote)

	// a busy locker skips silently
	mod.Locker.Lock()
	if err := mod.doSnapshotSync(rec); err != nil {
		t.Fatalf("doSnapshotSync under an active locker = %v, want nil", err)
	}
	mod.Locker.Unlock()

	// the snapshot sync fast-forwards exactly to the checkpoint
	if err := mod.doSnapshotSync(rec); err != nil {
		t.Fatalf("doSnapshotSync failed: %s", err)
	}
	if h := local.Chain.GetLatestBlockHeight(); h != 10 {
		t.Fatalf("height after the snapshot sync = %d, want the checkpoint 10", h)
	}
	if !bytes.Equal(local.Chain.GetLatestBlockHash(), remoteChain[9].GetHash()) {
		t.Fatal("local tip is not the remote checkpoint block")
	}

	// already at the checkpoint: nothing to do
	if err := mod.doSnapshotSync(rec); err != nil {
		t.Fatalf("second doSnapshotSync = %v, want nil", err)
	}
	if local.Chain.GetLatestBlockHeight() != 10 {
		t.Fatal("an up-to-date snapshot sync must not move the chain")
	}

	// a checkpoint beyond the remote's chain fails the range fetch
	farRec := NewRemoteRecord(remote.LocalNode.ID(), 0, 50,
		remote.Chain.GetLatestCheckpoint().GetHash(), []byte{0x01})
	if err := mod.doSnapshotSync(farRec); !errors.Is(err, ErrMsgRejected) {
		t.Fatalf("doSnapshotSync beyond the remote = %v, want ErrMsgRejected", err)
	}
}

// TestDoSnapshotConvergingExtend covers the branch where the fork
// segment ends below the remote checkpoint and gets extended by a
// range fetch
func TestDoSnapshotConvergingExtend(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	remoteKey, _ := ngtypes.GenerateKey()
	remoteChain := mineBlocks(t, remote, remoteKey, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true, SnapshotMode: true})
	localKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, local, localKey, 1) // 1-block fork, checkpoint at 10

	connectNodes(t, local, remote)
	mod := local.SyncMod
	rec := recordFor(remote)

	// a busy locker skips silently
	mod.Locker.Lock()
	if err := mod.doSnapshotConverging(rec); err != nil {
		t.Fatalf("doSnapshotConverging under an active locker = %v, want nil", err)
	}
	mod.Locker.Unlock()

	if err := mod.doSnapshotConverging(rec); err != nil {
		t.Fatalf("doSnapshotConverging failed: %s", err)
	}
	if h := local.Chain.GetLatestBlockHeight(); h != 10 {
		t.Fatalf("height after snapshot converging = %d, want the checkpoint 10", h)
	}
	if !bytes.Equal(local.Chain.GetLatestBlockHash(), remoteChain[9].GetHash()) {
		t.Fatal("local tip is not the remote checkpoint block")
	}
}

// TestDoSnapshotConvergingCut covers the branch where the fetched fork
// branch overshoots the remote checkpoint and gets trimmed back to it
func TestDoSnapshotConvergingCut(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	remoteKey, _ := ngtypes.GenerateKey()
	remoteChain := mineBlocks(t, remote, remoteKey, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true, SnapshotMode: true})
	localKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, local, localKey, 12) // 12-block fork above the checkpoint

	connectNodes(t, local, remote)
	mod := local.SyncMod

	if err := mod.doSnapshotConverging(recordFor(remote)); err != nil {
		t.Fatalf("doSnapshotConverging failed: %s", err)
	}
	if h := local.Chain.GetLatestBlockHeight(); h != 10 {
		t.Fatalf("height after snapshot converging = %d, want the checkpoint 10", h)
	}
	if !bytes.Equal(local.Chain.GetLatestBlockHash(), remoteChain[9].GetHash()) {
		t.Fatal("local tip is not the remote checkpoint block")
	}
}

func TestBootstrap(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	key, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, key, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	connectNodes(t, local, remote)

	local.SyncMod.bootstrap()

	if h := local.Chain.GetLatestBlockHeight(); h != 12 {
		t.Fatalf("height after bootstrap = %d, want 12", h)
	}
	if !bytes.Equal(local.Chain.GetLatestBlockHash(), remote.Chain.GetLatestBlockHash()) {
		t.Fatal("local tip does not match the remote tip after bootstrap")
	}
}

// TestSyncLoop runs the full background machinery on a forked local
// node: the loop pings the remote, fails the plain sync (forked tip),
// converges onto the remote branch and finishes the sync
func TestSyncLoop(t *testing.T) {
	remote := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	remoteKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, remote, remoteKey, 12)

	local := newTestNode(t, PoWorkConfig{DisableConnectingBootstraps: true})
	localKey, _ := ngtypes.GenerateKey()
	mineBlocks(t, local, localKey, 1) // fork to be abandoned by converging

	connectNodes(t, local, remote)

	local.GoLoop()

	waitUntil(t, 30*time.Second, func() bool {
		return local.Chain.GetLatestBlockHeight() == 12 &&
			bytes.Equal(local.Chain.GetLatestBlockHash(), remote.Chain.GetLatestBlockHash())
	}, "the sync loop never caught up with the remote")
}
