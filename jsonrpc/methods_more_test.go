package jsonrpc_test

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestRPCGetContractExports pins the ABI introspection: a contract's
// zero-arg exports are listed and marked callable
func TestRPCGetContractExports(t *testing.T) {
	const wat = `(module (memory 1) (func (export "main") nop) (func (export "ping") nop))`
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	mineViaRPC(t, node, key) // fund
	var unsigned string
	decodeInto(t, node.mustCall(t, "ng_genCommit", map[string]any{
		"fee": "0.05", "wasm": hex.EncodeToString(mustWat(wat)),
	}), &unsigned)
	commitReveal(t, node, key, unsigned)
	mineViaRPC(t, node, key)

	var exports []struct {
		Name     string `json:"name"`
		Callable bool   `json:"callable"`
	}
	decodeInto(t, node.mustCall(t, "ng_getContractExports", map[string]any{"address": addr.BS58()}), &exports)

	callable := map[string]bool{}
	for _, e := range exports {
		callable[e.Name] = e.Callable
	}
	if !callable["main"] || !callable["ping"] {
		t.Fatalf("exports = %+v, want main+ping callable", exports)
	}
}

// TestRPCBlockTxNavigation covers getBlockTransactionCount and
// getTransactionByBlockAndIndex
func TestRPCBlockTxNavigation(t *testing.T) {
	node := newRPCNode(t)
	miner, _ := ngtypes.GenerateKey()
	mineViaRPC(t, node, miner) // block @1 has the coinbase

	var count int
	decodeInto(t, node.mustCall(t, "ng_getBlockTransactionCount", map[string]any{"height": uint64(1)}), &count)
	if count < 1 {
		t.Fatalf("block tx count = %d, want >= 1", count)
	}

	// index 0 resolves; out-of-range errors
	node.mustCall(t, "ng_getTransactionByBlockAndIndex", map[string]any{"height": uint64(1), "index": 0})
	if _, rpcErr := node.call(t, "ng_getTransactionByBlockAndIndex", map[string]any{"height": uint64(1), "index": 99}); rpcErr == nil {
		t.Error("an out-of-range index must error")
	}
}

// TestRPCRemovePeer covers admin_removePeer: a bad id errors, a valid but
// unconnected id is a no-op success
func TestRPCRemovePeer(t *testing.T) {
	node := newRPCNode(t)

	if _, rpcErr := node.call(t, "admin_removePeer", map[string]any{"peerId": "not-a-peer"}); rpcErr == nil {
		t.Error("a malformed peer id must error")
	}

	priv, _, _ := crypto.GenerateEd25519Key(rand.Reader)
	strangerID, _ := peer.IDFromPrivateKey(priv)
	var ok bool
	decodeInto(t, node.mustCall(t, "admin_removePeer", map[string]any{"peerId": strangerID.String()}), &ok)
	if !ok {
		t.Error("removing an unconnected peer should succeed as a no-op")
	}
}

// TestRPCGetAddressStateAtHeight covers the historical getAddressState
func TestRPCGetAddressStateAtHeight(t *testing.T) {
	node := newRPCNode(t)
	node.pow.State.Archive = true
	miner, _ := ngtypes.GenerateKey()

	mineViaRPC(t, node, miner) // @1
	mineViaRPC(t, node, miner) // @2 (balance grows)

	var st struct {
		Balance string `json:"balance"`
		Exists  bool   `json:"exists"`
	}
	decodeInto(t, node.mustCall(t, "ng_getAddressState", map[string]any{
		"address": ngtypes.NewAddress(miner).BS58(), "height": uint64(1),
	}), &st)
	if st.Balance != ngtypes.GetBlockReward(1).String() {
		t.Fatalf("balance@1 = %s, want %s", st.Balance, ngtypes.GetBlockReward(1))
	}
	if !st.Exists {
		t.Error("miner should exist at height 1")
	}
}
