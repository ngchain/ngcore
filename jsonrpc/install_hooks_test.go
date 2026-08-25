package jsonrpc_test

import (
	"encoding/json"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestRPCPoolResetHookSurvivesInstall guards a hook-ownership hazard: consensus
// registers pool.Reset on Chain.OnTipChanged, and the WS hub's install() must
// COMPOSE with it rather than clobber it — otherwise, with the RPC server
// running, the mempool would never deprecate its height-locked txs on a tip
// change.
func TestRPCPoolResetHookSurvivesInstall(t *testing.T) {
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	mineViaRPC(t, node, key) // fund key

	// queue a tx in the mempool: commitReveal lands the commitment and
	// leaves the reveal pending (locked on the next height)
	var unsigned string
	decodeInto(t, node.mustCall(t, "ng_genTransaction", map[string]any{
		"to": ngtypes.NewAddress(key).BS58(), "value": "1", "fee": "0.01",
	}), &unsigned)
	commitReveal(t, node, key, unsigned)

	pendingCount := func() int {
		var reply struct {
			Count int               `json:"count"`
			Txs   []json.RawMessage `json:"txs"`
		}
		decodeInto(t, node.mustCall(t, "ng_getPendingTxs", nil), &reply)
		return reply.Count
	}
	if pendingCount() == 0 {
		t.Fatal("tx did not enter the pool")
	}

	// fire a tip change directly: the composed hook must still run pool.Reset
	node.pow.Chain.OnTipChanged()

	if n := pendingCount(); n != 0 {
		t.Fatalf("pool not reset on tip change: %d tx(s) remain (install clobbered pool.Reset)", n)
	}
}
