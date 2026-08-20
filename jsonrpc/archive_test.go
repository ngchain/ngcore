package jsonrpc_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestRPCArchiveBalanceHistory drives the archive read path end to end:
// with archive on, a miner's balance at a PAST height comes back as the
// value it held then, not the current tip
func TestRPCArchiveBalanceHistory(t *testing.T) {
	node := newRPCNode(t)
	node.pow.State.Archive = true // enable before any block is applied

	miner, _ := ngtypes.GenerateKey()
	minerAddr := ngtypes.NewAddress(miner).BS58()

	// three empty blocks: the miner's total balance grows by the block
	// reward each height (height 1, 2, 3)
	mineViaRPC(t, node, miner)
	mineViaRPC(t, node, miner)
	mineViaRPC(t, node, miner)

	balAt := func(height *uint64) *big.Int {
		params := map[string]any{"address": minerAddr}
		if height != nil {
			params["height"] = *height
		}
		var r struct {
			TotalBalance string `json:"TotalBalance"`
		}
		decodeInto(t, node.mustCall(t, "ng_getBalanceByAddress", params), &r)
		v, ok := new(big.Int).SetString(r.TotalBalance, 10)
		if !ok {
			t.Fatalf("bad balance %q", r.TotalBalance)
		}
		return v
	}

	h := func(x uint64) *uint64 { return &x }

	want1 := ngtypes.GetBlockReward(1)
	want2 := new(big.Int).Add(want1, ngtypes.GetBlockReward(2))
	want3 := new(big.Int).Add(want2, ngtypes.GetBlockReward(3))

	if got := balAt(h(1)); got.Cmp(want1) != 0 {
		t.Errorf("balance@1 = %s, want %s", got, want1)
	}
	if got := balAt(h(2)); got.Cmp(want2) != 0 {
		t.Errorf("balance@2 = %s, want %s", got, want2)
	}
	// tip (no height) equals the running total after three blocks
	if got := balAt(nil); got.Cmp(want3) != 0 {
		t.Errorf("tip balance = %s, want %s", got, want3)
	}

	// a height above the tip is rejected, not silently answered
	if _, rpcErr := node.call(t, "ng_getBalanceByAddress",
		map[string]any{"address": minerAddr, "height": uint64(99)}); rpcErr == nil {
		t.Error("a height above the tip must be a jsonrpc error")
	}
}

// TestRPCArchiveContractHistory drives the historical contract reads end
// to end: getContractInfo before the slot existed reports no slot, and
// getContractStorage reads a kv value as of a past height (empty before
// the contract's main ran, "val" at the tip)
func TestRPCArchiveContractHistory(t *testing.T) {
	const contractWat = `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))))
`
	node := newRPCNode(t)
	node.pow.State.Archive = true
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	latest := func() uint64 {
		var h uint64
		decodeInto(t, node.mustCall(t, "ng_getLatestBlockHeight", nil), &h)
		return h
	}
	send := func(method string, params any) {
		var unsigned string
		decodeInto(t, node.mustCall(t, method, params), &unsigned)
		node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": localSign(t, key, unsigned)})
	}
	h := func(x uint64) *uint64 { return &x }

	mineViaRPC(t, node, key) // fund the deployer
	hBeforeCommit := latest()

	send("ng_genCommit", map[string]any{"fee": "0.05", "wasm": hex.EncodeToString(mustWat(contractWat))})
	mineViaRPC(t, node, key)
	send("ng_genActivate", map[string]any{"fee": "0.05"})
	mineViaRPC(t, node, key)
	hBeforeRun := latest()

	send("ng_genTransaction", map[string]any{"to": addr.BS58(), "value": "0", "fee": "0.01"})
	mineViaRPC(t, node, key) // runs main -> sets key=val

	keyHex := hex.EncodeToString([]byte("key"))
	storageAt := func(height *uint64) string {
		params := map[string]any{"address": addr.BS58(), "key": keyHex}
		if height != nil {
			params["height"] = *height
		}
		var r struct {
			Value string `json:"value"`
		}
		decodeInto(t, node.mustCall(t, "ng_getContractStorage", params), &r)
		return r.Value
	}

	// before main ran, the slot exists but "key" is unset; at the tip it holds val
	if v := storageAt(h(hBeforeRun)); v != "" {
		t.Fatalf("storage@%d = %q, want empty (before main ran)", hBeforeRun, v)
	}
	if v := storageAt(nil); v != hex.EncodeToString([]byte("val")) {
		t.Fatalf("tip storage = %q, want hex(val)", v)
	}

	// getContractInfo before the commit: no slot existed at that height
	if _, rpcErr := node.call(t, "ng_getContractInfo",
		map[string]any{"address": addr.BS58(), "height": hBeforeCommit}); rpcErr == nil {
		t.Fatal("getContractInfo before the commit must report no slot at that height")
	}
}

// TestRPCArchiveDisabled pins that a node with archive explicitly OFF
// refuses historical reads rather than returning the current value
// (archive is on by default, so the test opts out)
func TestRPCArchiveDisabled(t *testing.T) {
	node := newRPCNode(t)
	node.pow.State.Archive = false // opt out of the default archive mode

	miner, _ := ngtypes.GenerateKey()
	mineViaRPC(t, node, miner)

	if _, rpcErr := node.call(t, "ng_getBalanceByAddress", map[string]any{
		"address": ngtypes.NewAddress(miner).BS58(),
		"height":  uint64(1),
	}); rpcErr == nil {
		t.Error("historical read on a non-archive node must be a jsonrpc error")
	}
}
