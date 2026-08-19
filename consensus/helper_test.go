package consensus

// helpers for the consensus tests: they boot real in-process full
// nodes (temp bbolt file + libp2p on loopback ephemeral ports), mine
// zeronet blocks (difficulty 1, instantly sealable) and wire nodes
// together, so every sync path runs against a genuine remote peer.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// newTestNode boots a full node: storage + chain + state + p2p (on an
// ephemeral loopback port, discovery off) + consensus
func newTestNode(t *testing.T, cfg PoWorkConfig) *PoWork {
	t.Helper()

	dir := t.TempDir()
	db, err := bbolt.Open(filepath.Join(dir, "chain.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	storage.InitDB(db)

	store := ngblocks.Init(db, ngtypes.ZERONET)
	state := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)
	chain := blockchain.Init(db, ngtypes.ZERONET, store, state)

	local := ngp2p.InitLocalNode(chain, ngp2p.P2PConfig{
		P2PKeyFile:                  filepath.Join(dir, "p2p.key"),
		Network:                     ngtypes.ZERONET,
		Port:                        0, // ephemeral
		DisableDiscovery:            true,
		DisableConnectingBootstraps: true,
	})
	local.GoServe()

	pool := ngpool.Init(db, chain, local)

	cfg.Network = ngtypes.ZERONET
	pow := InitPoWConsensus(db, chain, pool, state, local, cfg)

	t.Cleanup(func() {
		pow.Stop()
		time.Sleep(100 * time.Millisecond)
		_ = db.Close()
	})

	return pow
}

// newBarePow builds a PoWork around a real chain but without any p2p
// node, for the pure fork-choice/filter logic
func newBarePow(t *testing.T, network ngtypes.Network) *PoWork {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "chain.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	storage.InitDB(db)

	store := ngblocks.Init(db, network)
	state := ngstate.InitStateFromGenesis(db, network)
	chain := blockchain.Init(db, network, store, state)

	ctx, cancel := context.WithCancel(context.Background())
	pow := &PoWork{
		PoWorkConfig: PoWorkConfig{Network: network},
		Chain:        chain,
		State:        state,
		db:           db,
		orphans:      newOrphanPool(),
		ctx:          ctx,
		cancel:       cancel,
	}

	t.Cleanup(func() {
		cancel()
		_ = db.Close()
	})

	return pow
}

// sealBlock brute-forces a nonce; on zeronet (difficulty 1) the first
// nonce already meets the target
func sealBlock(t *testing.T, b *ngtypes.FullBlock) *ngtypes.FullBlock {
	t.Helper()

	for n := uint64(0); n < 1_000_000; n++ {
		nonce := utils.PackUint64LE(n)
		if err := b.ToSealed(nonce); err != nil {
			t.Fatal(err)
		}
		if b.CheckError() == nil {
			return b
		}
	}

	t.Fatal("failed to seal the block template")
	return nil
}

// mineBlocks mines and applies n blocks on top of the node's tip.
// NOTE: back-to-back templates advance the timestamp by 1s each, so n
// must stay below ngtypes.TimestampDriftTolerance (15)
func mineBlocks(t *testing.T, pow *PoWork, key *ngtypes.PrivateKey, n int) []*ngtypes.FullBlock {
	t.Helper()

	blocks := make([]*ngtypes.FullBlock, 0, n)
	for i := 0; i < n; i++ {
		b := sealBlock(t, pow.GetBlockTemplate(key).(*ngtypes.FullBlock))
		if err := pow.Chain.ApplyBlock(b); err != nil {
			t.Fatalf("failed to apply mined block@%d: %s", b.GetHeight(), err)
		}
		blocks = append(blocks, b)
	}

	return blocks
}

// connectNodes dials from -> to and waits until identify announced the
// wired protocol, so the sync module can discover the peer
func connectNodes(t *testing.T, from, to *PoWork) {
	t.Helper()

	err := from.LocalNode.Connect(context.Background(), peer.AddrInfo{
		ID:    to.LocalNode.ID(),
		Addrs: to.LocalNode.Addrs(),
	})
	if err != nil {
		t.Fatalf("failed to connect the test nodes: %s", err)
	}

	waitUntil(t, 10*time.Second, func() bool {
		p, _ := from.LocalNode.Peerstore().FirstSupportedProtocol(
			to.LocalNode.ID(), from.LocalNode.GetWiredProtocol())
		return p == from.LocalNode.GetWiredProtocol()
	}, "identify never announced the wired protocol")
}

// recordFor builds the RemoteRecord a ping/pong roundtrip with the
// remote node would produce
func recordFor(remote *PoWork) *RemoteRecord {
	cp := remote.Chain.GetLatestCheckpoint()
	return NewRemoteRecord(
		remote.LocalNode.ID(),
		remote.Chain.GetOriginBlock().GetHeight(),
		remote.Chain.GetLatestBlockHeight(),
		cp.GetHash(),
		cp.GetActualDiff().Bytes(),
	)
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("condition not met within %s: %s", timeout, msg)
}
