package jsonrpc_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestRPCGetLogs drives ng_getLogs end to end: a contract that emits a
// user event AND makes a native transfer must surface BOTH — the user log
// and the auto-emitted ng.transfer "internal transaction" — with working
// emitter/topic filters
func TestRPCGetLogs(t *testing.T) {
	// main: set kv "key"="val", emit topic "key" data "val", then
	// transfer value 1 to the zero address (to@32 is zero memory, value@64=1)
	const contractWat = `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (import "log" "emit" (func $emit (param i32 i32 i32 i32) (result i32)))
  (import "coin" "transfer" (func $transfer (param i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (data (i32.const 64) "\01")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))
    (drop (call $emit (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))
    (drop (call $transfer (i32.const 32) (i32.const 64)))))
`
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	send := func(method string, params any) {
		var unsigned string
		decodeInto(t, node.mustCall(t, method, params), &unsigned)
		node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": localSign(t, key, unsigned)})
	}

	mineViaRPC(t, node, key) // fund the deployer
	send("ng_genCommit", map[string]any{"fee": "0.05", "wasm": hex.EncodeToString(mustWat(contractWat))})
	mineViaRPC(t, node, key)
	send("ng_genActivate", map[string]any{"fee": "0.05"})
	mineViaRPC(t, node, key)
	send("ng_genTransaction", map[string]any{"to": addr.BS58(), "value": "0", "fee": "0.01"})
	mineViaRPC(t, node, key) // runs main -> emits both logs

	type log struct {
		Height   uint64 `json:"height"`
		Contract string `json:"contract"`
		Topic    string `json:"topic"`
		Data     string `json:"data"`
	}
	getLogs := func(params map[string]any) []log {
		var out []log
		decodeInto(t, node.mustCall(t, "ng_getLogs", params), &out)
		return out
	}

	// unfiltered over the whole chain: both logs present, both emitted by addr
	all := getLogs(map[string]any{"fromHeight": 0})
	var sawUser, sawTransfer bool
	for _, l := range all {
		if l.Contract != addr.BS58() {
			continue
		}
		switch l.Topic {
		case "key":
			sawUser = l.Data == hex.EncodeToString([]byte("val"))
		case "ng.transfer":
			// data = to(32 zero) ‖ value(32 LE, =1)
			want := strings.Repeat("00", 32) + "01" + strings.Repeat("00", 31)
			sawTransfer = l.Data == want
		}
	}
	if !sawUser {
		t.Error("ng_getLogs did not surface the user event (topic key)")
	}
	if !sawTransfer {
		t.Error("ng_getLogs did not surface the internal transfer (topic ng.transfer)")
	}

	// topic filter isolates the internal transfer
	only := getLogs(map[string]any{"fromHeight": 0, "topic": "ng.transfer"})
	if len(only) != 1 || only[0].Topic != "ng.transfer" {
		t.Fatalf("topic filter returned %d logs, want 1 ng.transfer", len(only))
	}

	// emitter filter on an unrelated address returns nothing
	other, _ := ngtypes.GenerateKey()
	if got := getLogs(map[string]any{"fromHeight": 0, "address": ngtypes.NewAddress(other).BS58()}); len(got) != 0 {
		t.Fatalf("unrelated address returned %d logs, want 0", len(got))
	}
}
