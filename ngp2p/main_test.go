package ngp2p

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p/wired"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// All tests here are hermetic: discovery runs against an EMPTY or
// loopback-only bootstrap list, never the public one.

func newTestChain(t *testing.T) *blockchain.Chain {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "chain.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storage.InitDB(db)
	store := ngblocks.Init(db, ngtypes.ZERONET)
	state := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)

	return blockchain.Init(db, ngtypes.ZERONET, store, state)
}

// newTestNode boots a LocalNode on random loopback-capable ports.
// NOTE: InitLocalNode never stores the config into LocalNode.P2PConfig
// (suspected bug, see the report), so the tests re-set it manually for
// the peer manager
func newTestNode(t *testing.T, cfg P2PConfig) *LocalNode {
	t.Helper()

	if cfg.P2PKeyFile == "" {
		cfg.P2PKeyFile = filepath.Join(t.TempDir(), "ngp2p.key")
	}

	node := InitLocalNode(newTestChain(t), cfg)
	node.P2PConfig = cfg

	t.Cleanup(func() { _ = node.Close() })

	return node
}

func hermeticConfig() P2PConfig {
	return P2PConfig{
		Network:                     ngtypes.ZERONET,
		Port:                        0, // random free port
		DisableDiscovery:            false,
		DisableConnectingBootstraps: true, // never dial the public bootstraps
	}
}

// loopbackAddr returns the node's 127.0.0.1 listen address with the
// /p2p/<id> component attached
func loopbackAddr(t *testing.T, node *LocalNode) multiaddr.Multiaddr {
	t.Helper()

	addrs, err := node.Network().InterfaceListenAddresses()
	if err != nil {
		t.Fatal(err)
	}

	p2pPart, err := multiaddr.NewMultiaddr("/p2p/" + node.ID().String())
	if err != nil {
		t.Fatal(err)
	}

	for _, addr := range addrs {
		if strings.HasPrefix(addr.String(), "/ip4/127.0.0.1/tcp/") {
			return addr.Encapsulate(p2pPart)
		}
	}

	t.Fatal("no loopback listen address found")
	return nil
}

func connectNodes(t *testing.T, from, to *LocalNode) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pi, err := peer.AddrInfoFromP2pAddr(loopbackAddr(t, to))
	if err != nil {
		t.Fatal(err)
	}

	if err := from.Connect(ctx, *pi); err != nil {
		t.Fatal(err)
	}
}

func TestInitLocalNodeHermetic(t *testing.T) {
	node := newTestNode(t, hermeticConfig())
	node.GoServe()

	if node.ID() == "" {
		t.Error("node must have an identity")
	}
	if len(node.Addrs()) == 0 {
		t.Error("node must listen somewhere")
	}
	if node.GetWiredProtocol() == "" {
		t.Error("wired protocol must be set")
	}

	if err := node.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestInitLocalNodeDisabledDiscovery(t *testing.T) {
	cfg := hermeticConfig()
	cfg.DisableDiscovery = true

	node := newTestNode(t, cfg)
	if node.ID() == "" {
		t.Error("node must have an identity")
	}
}

func TestKeyFilePersistence(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "ngp2p.key")

	cfg := hermeticConfig()
	cfg.DisableDiscovery = true
	cfg.P2PKeyFile = keyFile

	node1 := newTestNode(t, cfg)
	id1 := node1.ID()
	if err := node1.Close(); err != nil {
		t.Fatal(err)
	}

	// same key file must yield the same identity
	node2 := newTestNode(t, cfg)
	if node2.ID() != id1 {
		t.Errorf("identity changed: %s != %s", node2.ID(), id1)
	}
}

func TestWiredPingOverLocalNodes(t *testing.T) {
	node1 := newTestNode(t, hermeticConfig())
	node2 := newTestNode(t, hermeticConfig())
	node1.GoServe()
	node2.GoServe()

	connectNodes(t, node2, node1)

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)

	id, stream := node2.SendPing(node1.ID(), 0, 0,
		genesis.GetHash(), genesis.GetActualDiff().Bytes())
	if id == nil || stream == nil {
		t.Fatal("SendPing failed")
	}

	_ = stream.SetDeadline(time.Now().Add(10 * time.Second))
	msg, err := wired.ReceiveReply(id, stream)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Header.Type != wired.PongMsg {
		t.Fatalf("expected PongMsg, got %s (payload %q)", msg.Header.Type, msg.Payload)
	}
}

func TestConnectToDHTBootstrapNodes(t *testing.T) {
	node1 := newTestNode(t, hermeticConfig())
	node2 := newTestNode(t, hermeticConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// an unreachable loopback bootstrap must not connect
	dead, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/1/p2p/" + node1.ID().String())
	if err != nil {
		t.Fatal(err)
	}
	if n := connectToDHTBootstrapNodes(ctx, node2, []multiaddr.Multiaddr{dead}); n != 0 {
		t.Errorf("connected to %d unreachable bootstraps", n)
	}

	// a live loopback bootstrap must connect
	if n := connectToDHTBootstrapNodes(ctx, node2, []multiaddr.Multiaddr{loopbackAddr(t, node1)}); n != 1 {
		t.Errorf("connected to %d/1 live bootstraps", n)
	}
}

// TestInitLocalNodeConnectsBootstraps swaps the global bootstrap list
// for a loopback-only one and verifies the bootstrap dialing path of
// activeDHT, staying entirely on 127.0.0.1
func TestInitLocalNodeConnectsBootstraps(t *testing.T) {
	node1 := newTestNode(t, hermeticConfig())
	node1.GoServe()

	oldBootstraps := BootstrapNodes
	BootstrapNodes = []multiaddr.Multiaddr{loopbackAddr(t, node1)}
	defer func() { BootstrapNodes = oldBootstraps }()

	cfg := hermeticConfig()
	cfg.DisableConnectingBootstraps = false // safe: the list is loopback-only

	node2 := newTestNode(t, cfg)

	if node2.Network().Connectedness(node1.ID()) != network.Connected {
		t.Error("node2 must have connected to the loopback bootstrap node")
	}
}

func TestPeerManagerRedials(t *testing.T) {
	cfg := hermeticConfig()
	cfg.DisableDiscovery = true
	cfg.MinPeers = 1
	cfg.ReconnectInterval = 100 * time.Millisecond

	node1 := newTestNode(t, cfg)
	node2 := newTestNode(t, cfg)
	node1.GoServe()
	node2.GoServe()

	connectNodes(t, node2, node1)
	if node2.Network().Connectedness(node1.ID()) != network.Connected {
		t.Fatal("nodes must be connected")
	}

	// drop the connection: the peer manager must redial from the peerstore
	if err := node2.Network().ClosePeer(node1.ID()); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for node2.Network().Connectedness(node1.ID()) != network.Connected {
		if time.Now().After(deadline) {
			t.Fatal("the peer manager never redialed the known peer")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// let a tick observe the satisfied MinPeers state as well
	time.Sleep(300 * time.Millisecond)
}

func TestRedialKnownPeersBootstrapBranch(t *testing.T) {
	cfg := hermeticConfig()
	cfg.DisableDiscovery = true
	cfg.DisableConnectingBootstraps = false // exercise the bootstrap redial branch
	cfg.MinPeers = 1

	node := newTestNode(t, cfg)
	// NOTE: no GoServe — redialKnownPeers is driven synchronously below,
	// so nothing else reads the swapped global list

	noP2P, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/1") // no /p2p/: skipped
	if err != nil {
		t.Fatal(err)
	}
	dead, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/1/p2p/" + node.ID().String())
	if err != nil {
		t.Fatal(err)
	}

	oldBootstraps := BootstrapNodes
	BootstrapNodes = []multiaddr.Multiaddr{noP2P, dead}
	defer func() { BootstrapNodes = oldBootstraps }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	node.redialKnownPeers(ctx) // must survive invalid + unreachable entries

	// give the async dials a moment before the bootstrap list is restored
	time.Sleep(200 * time.Millisecond)
}
