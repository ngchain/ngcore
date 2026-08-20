package jsonrpc_test

import (
	"encoding/hex"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestRPCTraceTransaction drives ng_traceTransaction end to end: a
// contract that makes a native transfer must expose that transfer as a
// trace frame (an internal transaction) attributed to the sender
func TestRPCTraceTransaction(t *testing.T) {
	const contractWat = `
(module
  (import "coin" "transfer" (func $transfer (param i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 64) "\01")
  (func (export "main")
    (drop (call $transfer (i32.const 32) (i32.const 64)))))
`
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	send := func(method string, params any) string {
		var unsigned string
		decodeInto(t, node.mustCall(t, method, params), &unsigned)
		var txHash string
		decodeInto(t, node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": localSign(t, key, unsigned)}), &txHash)
		return txHash
	}

	mineViaRPC(t, node, key)
	send("ng_genCommit", map[string]any{"fee": "0.05", "wasm": hex.EncodeToString(mustWat(contractWat))})
	mineViaRPC(t, node, key)
	send("ng_genActivate", map[string]any{"fee": "0.05"})
	mineViaRPC(t, node, key)
	txHash := send("ng_genTransaction", map[string]any{"to": addr.BS58(), "value": "0", "fee": "0.01"})
	mineViaRPC(t, node, key) // runs main -> internal transfer

	var trace struct {
		OnChain     bool   `json:"onChain"`
		BlockHeight uint64 `json:"blockHeight"`
		Runs        []struct {
			Ok    bool `json:"ok"`
			Trace []struct {
				Type  string `json:"type"`
				Depth uint32 `json:"depth"`
				From  string `json:"from"`
				Ok    bool   `json:"ok"`
			} `json:"trace"`
		} `json:"runs"`
	}
	decodeInto(t, node.mustCall(t, "ng_traceTransaction", map[string]any{"hash": txHash}), &trace)

	if !trace.OnChain || len(trace.Runs) == 0 {
		t.Fatalf("trace = %+v, want an on-chain run", trace)
	}
	var found bool
	for _, run := range trace.Runs {
		for _, fr := range run.Trace {
			if fr.Type == "transfer" && fr.Depth == 0 && fr.Ok && fr.From == addr.BS58() {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("ng_traceTransaction did not expose the internal transfer: %+v", trace.Runs)
	}

	// ng_traceBlock over the same block must contain that tx's trace
	var blk struct {
		Height uint64 `json:"height"`
		Txs    []struct {
			TxHash string `json:"txHash"`
			Runs   []struct {
				Trace []struct {
					Type string `json:"type"`
				} `json:"trace"`
			} `json:"runs"`
		} `json:"txs"`
	}
	decodeInto(t, node.mustCall(t, "ng_traceBlock", map[string]any{"height": trace.BlockHeight}), &blk)
	var blockHasTransfer bool
	for _, tx := range blk.Txs {
		if tx.TxHash != txHash {
			continue
		}
		for _, run := range tx.Runs {
			for _, fr := range run.Trace {
				if fr.Type == "transfer" {
					blockHasTransfer = true
				}
			}
		}
	}
	if !blockHasTransfer {
		t.Fatalf("ng_traceBlock(%d) missing the tx's transfer trace: %+v", trace.BlockHeight, blk.Txs)
	}
}
