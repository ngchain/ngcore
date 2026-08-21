package jsonrpc_test

import (
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestRPCNodeStatus covers ng_syncing and ng_suggestFee
func TestRPCNodeStatus(t *testing.T) {
	node := newRPCNode(t)

	// a fresh in-process node is caught up, tip at genesis
	var sync struct {
		Syncing bool   `json:"syncing"`
		Height  uint64 `json:"height"`
	}
	decodeInto(t, node.mustCall(t, "ng_syncing", nil), &sync)
	if sync.Syncing {
		t.Error("a fresh node must not report syncing")
	}
	if sync.Height != 0 {
		t.Errorf("syncing height = %d, want 0", sync.Height)
	}

	// the per-byte relay floor, and the exact floor for a given rawTx
	var fee struct {
		MinFeePerByte string `json:"minFeePerByte"`
		MinFee        string `json:"minFee"`
	}
	decodeInto(t, node.mustCall(t, "ng_suggestFee", nil), &fee)
	if fee.MinFeePerByte != "10000000000" {
		t.Fatalf("minFeePerByte = %q, want 10000000000", fee.MinFeePerByte)
	}
	// a 5-byte rawTx: floor = 10000000000 * 5
	decodeInto(t, node.mustCall(t, "ng_suggestFee", map[string]any{"rawTx": "0011223344"}), &fee)
	if fee.MinFee != "50000000000" {
		t.Fatalf("minFee = %q, want 50000000000", fee.MinFee)
	}
}

// TestRPCObservabilityRejections sweeps the input-validation seams of the
// new observability/node methods
func TestRPCObservabilityRejections(t *testing.T) {
	node := newRPCNode(t)

	for _, c := range []struct {
		name   string
		method string
		params any
	}{
		{"getLogs/bs58", "ng_getLogs", map[string]any{"fromHeight": 0, "address": "0OIl"}},
		{"traceTransaction/hex", "ng_traceTransaction", map[string]any{"hash": "zz"}},
		{"traceBlock/aboveTip", "ng_traceBlock", map[string]any{"height": uint64(99)}},
		{"suggestFee/hex", "ng_suggestFee", map[string]any{"rawTx": "zz"}},
	} {
		if _, rpcErr := node.call(t, c.method, c.params); rpcErr == nil {
			t.Errorf("%s: accepted %v, want a jsonrpc error", c.name, c.params)
		}
	}
}

// TestRPCPendingTxs pins ng_getPendingTxs: empty at first, then it lists a
// broadcast-but-unmined tx
func TestRPCPendingTxs(t *testing.T) {
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	mineViaRPC(t, node, key) // fund the sender

	var empty struct {
		Count int `json:"count"`
	}
	decodeInto(t, node.mustCall(t, "ng_getPendingTxs", nil), &empty)
	if empty.Count != 0 {
		t.Fatalf("fresh pool count = %d, want 0", empty.Count)
	}

	var unsigned string
	decodeInto(t, node.mustCall(t, "ng_genTransaction", map[string]any{
		"to":    ngtypes.NewAddress(key).BS58(),
		"value": "1",
		"fee":   "0.01",
	}), &unsigned)
	node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": localSign(t, key, unsigned)})

	var pending struct {
		Count int `json:"count"`
		Txs   []struct {
			Type int `json:"type"`
		} `json:"txs"`
	}
	decodeInto(t, node.mustCall(t, "ng_getPendingTxs", nil), &pending)
	if pending.Count != 1 || len(pending.Txs) != 1 {
		t.Fatalf("pending = %+v, want 1 tx", pending)
	}
}
