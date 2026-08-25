package jsonrpc_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// TestRPCGetSheetAtHeight proves the scratch reconstruction rebuilds the
// correct FULL state at a past height — and works on a NON-archive node
// (it replays blocks, it does not read changesets)
func TestRPCGetSheetAtHeight(t *testing.T) {
	node := newRPCNode(t)
	node.pow.State.Archive = false // scratch reconstruction needs no archive

	miner, _ := ngtypes.GenerateKey()
	minerAddr := ngtypes.NewAddress(miner)
	mineViaRPC(t, node, miner) // @1
	mineViaRPC(t, node, miner) // @2

	balanceAt := func(height uint64) *ngtypes.Sheet {
		var r struct {
			Height uint64 `json:"height"`
			Sheet  string `json:"sheet"`
		}
		decodeInto(t, node.mustCall(t, "ng_getSheet", map[string]any{"height": height}), &r)
		if r.Height != height {
			t.Fatalf("sheet height = %d, want %d", r.Height, height)
		}
		var sheet ngtypes.Sheet
		if err := utils.HexRLPDecode(r.Sheet, &sheet); err != nil {
			t.Fatalf("sheet does not decode: %v", err)
		}
		return &sheet
	}

	minerBalance := func(sheet *ngtypes.Sheet) string {
		for _, b := range sheet.Balances {
			if b.Address == minerAddr {
				return b.Amount.String()
			}
		}
		return "0"
	}

	// at height 1 the miner has one reward; at height 2, two
	if got, want := minerBalance(balanceAt(1)), ngtypes.GetBlockReward(1).String(); got != want {
		t.Fatalf("sheet@1 miner balance = %s, want %s", got, want)
	}
	want2 := new(big.Int).Add(ngtypes.GetBlockReward(1), ngtypes.GetBlockReward(2)).String()
	if got := minerBalance(balanceAt(2)); got != want2 {
		t.Fatalf("sheet@2 miner balance = %s, want %s", got, want2)
	}
}

// TestRPCCallContractAtHeight proves a dry-run against reconstructed past
// state: the contract runs at a height where it is active, and the call is
// refused at a height before it existed
func TestRPCCallContractAtHeight(t *testing.T) {
	const contractWat = `
(module
  (import "log" "emit" (func $emit (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $emit (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))))
`
	node := newRPCNode(t)
	node.pow.State.Archive = false
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	gen := func(method string, params any) string {
		var unsigned string
		decodeInto(t, node.mustCall(t, method, params), &unsigned)
		return unsigned
	}
	mineViaRPC(t, node, key) // @1 fund
	// commitReveal mines the commit (@2), the reveal lands on @3
	commitReveal(t, node, key, gen("ng_genCommit", map[string]any{"fee": "0.05", "wasm": hex.EncodeToString(mustWat(contractWat))}))
	mineViaRPC(t, node, key) // @3 deploy goes live

	// at height 3 the contract is live: the dry-run runs and emits "key"
	var run struct {
		Ok     bool `json:"ok"`
		Events []struct {
			Topic string `json:"topic"`
		} `json:"events"`
	}
	decodeInto(t, node.mustCall(t, "ng_callContract", map[string]any{
		"contract": addr.BS58(), "height": uint64(3),
	}), &run)
	if !run.Ok {
		t.Fatalf("historical dry-run failed: %+v", run)
	}
	var emitted bool
	for _, e := range run.Events {
		if e.Topic == "key" {
			emitted = true
		}
	}
	if !emitted {
		t.Fatalf("historical dry-run missed the emit: %+v", run.Events)
	}

	// before the contract existed (height 1) the call is refused
	if _, rpcErr := node.call(t, "ng_callContract", map[string]any{
		"contract": addr.BS58(), "height": uint64(1),
	}); rpcErr == nil {
		t.Error("call at a height before deployment must error")
	}
}
