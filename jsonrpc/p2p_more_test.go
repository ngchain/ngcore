package jsonrpc_test

import (
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestRPCAddPeer drives the p2p management methods against a real
// second libp2p host in the same process: addPeer must connect, and
// getPeers must then list the new peer
func TestRPCAddPeer(t *testing.T) {
	node := newRPCNode(t)

	// a bare libp2p host is enough of a peer for Connect to succeed
	peerHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	defer peerHost.Close()

	target := peerHost.Addrs()[0].String() + "/p2p/" + peerHost.ID().String()
	node.mustCall(t, "addPeer", map[string]any{"peerMultiAddr": target})

	// the peerstore must now know the peer
	found := false
	for _, id := range node.pow.LocalNode.Peerstore().PeersWithAddrs() {
		if id == peerHost.ID() {
			found = true
		}
	}
	if !found {
		t.Fatal("addPeer connected but the peerstore does not list the peer")
	}

	// getPeers (and its alias getNodes) must answer without error
	node.mustCall(t, "getPeers", nil)
	node.mustCall(t, "getNodes", nil)

	// the addNode alias goes through the same handler
	node.mustCall(t, "addNode", map[string]any{"peerMultiAddr": target})

	// not a multiaddr at all
	if _, rpcErr := node.call(t, "addPeer",
		map[string]any{"peerMultiAddr": "not-a-multiaddr"}); rpcErr == nil {
		t.Fatal("addPeer must reject a malformed multiaddr")
	}

	// a multiaddr without the /p2p/<id> component has no peer info
	if _, rpcErr := node.call(t, "addPeer",
		map[string]any{"peerMultiAddr": "/ip4/127.0.0.1/tcp/1"}); rpcErr == nil {
		t.Fatal("addPeer must reject a multiaddr without a peer id")
	}

	// a well-formed but unreachable peer: the dial must fail
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	strangerID, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, rpcErr := node.call(t, "addPeer", map[string]any{
		"peerMultiAddr": "/ip4/127.0.0.1/tcp/1/p2p/" + strangerID.String(),
	}); rpcErr == nil {
		t.Fatal("addPeer must fail when the peer is unreachable")
	}
}
