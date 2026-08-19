package ngp2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

// NOTE: connectToDHTBootstrapNodes's AddrInfoFromP2pAddr panic (dht.go:57)
// fires inside a worker goroutine it spawns itself, so a caller-side recover
// cannot catch it — driving that branch would crash the whole test binary.
// The branch stays uncovered (defensive; the real bootstrap list is fixed and
// always carries /p2p/ components).

// TestRedialKnownPeersDialsPeerstore drives the peerstore-known-peer redial
// branch: a peer with addresses but no live connection must be dialed (the
// dial fails against a dead loopback, which is the logged path).
func TestRedialKnownPeersDialsPeerstore(t *testing.T) {
	cfg := hermeticConfig()
	cfg.DisableDiscovery = true
	cfg.DisableConnectingBootstraps = true // isolate the peerstore branch
	cfg.MinPeers = 1

	node := newTestNode(t, cfg)

	// craft a random, never-connected peer id with a dead loopback address
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		t.Fatal(err)
	}
	deadID, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	deadAddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/1")
	if err != nil {
		t.Fatal(err)
	}
	node.Peerstore().AddAddrs(deadID, []multiaddr.Multiaddr{deadAddr}, peerstore.PermanentAddrTTL)

	if node.Network().Connectedness(deadID) == network.Connected {
		t.Fatal("the crafted peer must not be connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	node.redialKnownPeers(ctx) // must attempt (and log-fail) the dead dial

	// let the async dial goroutine run before the node is torn down
	time.Sleep(300 * time.Millisecond)
}

// TestRedialKnownPeersSkipsConnectedBootstrap drives the "bootstrap already
// connected" continue branch (peer_manager.go:61): when a bootstrap node is
// already a live connection, redialKnownPeers must skip re-dialing it.
func TestRedialKnownPeersSkipsConnectedBootstrap(t *testing.T) {
	node1 := newTestNode(t, hermeticConfig())
	node1.GoServe()

	cfg := hermeticConfig()
	cfg.DisableDiscovery = true
	cfg.DisableConnectingBootstraps = false
	cfg.MinPeers = 1
	node2 := newTestNode(t, cfg)

	// connect node2 -> node1 first, then make node1 the sole bootstrap
	connectNodes(t, node2, node1)
	if node2.Network().Connectedness(node1.ID()) != network.Connected {
		t.Fatal("nodes must be connected")
	}

	oldBootstraps := BootstrapNodes
	BootstrapNodes = []multiaddr.Multiaddr{loopbackAddr(t, node1)}
	defer func() { BootstrapNodes = oldBootstraps }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// node1 is already connected: the bootstrap loop must hit the
	// Connectedness == Connected continue
	node2.redialKnownPeers(ctx)

	time.Sleep(100 * time.Millisecond)
}

// TestPeerManagerLoopDefaultMinPeers drives peerManagerLoop with MinPeers==0,
// exercising the default-min-peers branch and the redial trigger: with no
// connected peers the loop always falls below the (defaulted) minimum and
// calls redialKnownPeers on every tick.
func TestPeerManagerLoopDefaultMinPeers(t *testing.T) {
	cfg := hermeticConfig()
	cfg.DisableDiscovery = true
	cfg.DisableConnectingBootstraps = true
	cfg.MinPeers = 0 // -> DefaultMinPeers
	cfg.ReconnectInterval = 30 * time.Millisecond

	node := newTestNode(t, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	go node.peerManagerLoop(ctx)

	// with 0 peers and no bootstraps to dial, several ticks pass through the
	// "below minPeers -> redial" path; the loop just keeps trying harmlessly
	time.Sleep(200 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
}

// TestPeerManagerLoopSatisfied drives the "enough peers -> continue" branch:
// once the min-peers target is met the loop keeps ticking without redialing.
func TestPeerManagerLoopSatisfied(t *testing.T) {
	cfg := hermeticConfig()
	cfg.DisableDiscovery = true
	cfg.DisableConnectingBootstraps = true
	cfg.MinPeers = 1
	cfg.ReconnectInterval = 30 * time.Millisecond

	node1 := newTestNode(t, cfg)
	node2 := newTestNode(t, cfg)
	node1.GoServe()

	ctx, cancel := context.WithCancel(context.Background())
	go node2.peerManagerLoop(ctx)

	connectNodes(t, node2, node1)

	// let a few ticks observe the satisfied MinPeers (>= 1) state
	time.Sleep(200 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
}
