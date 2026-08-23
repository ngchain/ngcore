package jsonrpc_test

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// sealBlockOn builds and seals a ZERONET block on the given parent, carrying
// the coinbase plus any extra txs, so a test can grow a competing branch off
// an arbitrary fork point. Its timestamp follows the parent by one second —
// competing blocks are mined in-process, so they must stay within the
// future-drift tolerance.
func sealBlockOn(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey, extra ...*ngtypes.FullTx) *ngtypes.FullBlock {
	t.Helper()

	height := parent.GetHeight() + 1
	blockTime := parent.GetTimestamp() + 1
	diff := ngtypes.GetNextDiff(height, blockTime, parent)
	block := ngtypes.NewBareBlock(ngtypes.ZERONET, height, blockTime, parent.GetHash(), diff)
	block.SetCoinbase(ngtypes.NewAddress(miner))

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner), ngtypes.GetBlockReward(height), big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	if err := block.ToUnsealing(append([]*ngtypes.FullTx{genTx}, extra...)); err != nil {
		t.Fatal(err)
	}

	var last error
	for n := uint64(0); n < 2_000_000; n++ {
		if err := block.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if last = block.CheckError(); last == nil {
			return block
		}
	}
	t.Fatalf("failed to seal a competing ZERONET block@%d (diff=%s): last err %v",
		height, diff, last)
	return nil
}

// TestRPCWebSocketLogsRemovedOnReorg drives both sides of the logs
// subscription reorg path. A heavier branch orphans a contract emit (pushed
// again marked removed, so an indexer rolls it back) and, in a NON-tip block,
// re-emits (pushed marked not-removed, so the indexer replays it — this
// exercises the below-the-tip range that onTipChanged does not cover).
func TestRPCWebSocketLogsRemovedOnReorg(t *testing.T) {
	const contractWat = `
(module
  (import "log" "emit" (func $emit (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $emit (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))))
`
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	conn := dialWS(t, node)
	resp := wsCall(t, conn, 1, "ng_subscribe", map[string]any{
		"type": "logs", "address": addr.BS58(), "topic": "key",
	})
	var subID string
	if err := json.Unmarshal(resp["result"], &subID); err != nil || subID == "" {
		t.Fatalf("logs subscribe = %s (%v)", resp["result"], err)
	}

	send := func(method string, params any) {
		var unsigned string
		decodeInto(t, node.mustCall(t, method, params), &unsigned)
		node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": localSign(t, key, unsigned)})
	}
	mineViaRPC(t, node, key) // fund
	send("ng_genCommit", map[string]any{"fee": "0.05", "wasm": hex.EncodeToString(mustWat(contractWat))})
	mineViaRPC(t, node, key)
	send("ng_genActivate", map[string]any{"fee": "0.05"})
	mineViaRPC(t, node, key)

	// the tip after activation is the fork point: the emit block builds on it,
	// the competing branch replaces it
	forkParent, ok := node.pow.Chain.GetLatestBlock().(*ngtypes.FullBlock)
	if !ok {
		t.Fatal("latest block is not a *FullBlock")
	}

	// build the transact tx that runs the contract, and keep a copy: the
	// canonical chain mines it now, the competing branch reuses it (same
	// height, same fork-point state, so it stays valid) to emit off-tip
	var unsigned string
	decodeInto(t, node.mustCall(t, "ng_genTransaction", map[string]any{
		"to": addr.BS58(), "value": "0", "fee": "0.01",
	}), &unsigned)
	signedHex := localSign(t, key, unsigned)
	node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": signedHex})

	rawTx, err := hex.DecodeString(signedHex)
	if err != nil {
		t.Fatal(err)
	}
	var transactTx ngtypes.FullTx
	if err := rlp.DecodeBytes(rawTx, &transactTx); err != nil {
		t.Fatal(err)
	}

	mineViaRPC(t, node, key) // runs main -> emits the "key" log

	// drain the live emit notification (removed=false)
	readSub := func() (removed bool, topic string) {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var notif struct {
			Method string `json:"method"`
			Params struct {
				Result struct {
					Topic   string `json:"topic"`
					Removed bool   `json:"removed"`
				} `json:"result"`
			} `json:"params"`
		}
		for {
			if err := conn.ReadJSON(&notif); err != nil {
				t.Fatalf("ws read: %v", err)
			}
			if notif.Method == "ng_subscription" {
				return notif.Params.Result.Removed, notif.Params.Result.Topic
			}
		}
	}
	if removed, topic := readSub(); removed || topic != "key" {
		t.Fatalf("live emit notification = {removed:%v topic:%q}, want {false key}", removed, topic)
	}

	// grow a heavier branch off the fork point, mined by a DIFFERENT miner so
	// its blocks never collide with the canonical ones: b1 carries the same
	// transact (so it re-emits below the new tip), b2 is the empty tip that
	// out-works the single canonical emit block, forcing the reorg
	miner2, _ := ngtypes.GenerateKey()
	b1 := sealBlockOn(t, forkParent, miner2, &transactTx)
	if err := node.pow.Chain.ApplyBlock(b1); err != nil {
		t.Fatalf("apply competing b1: %v", err)
	}
	b2 := sealBlockOn(t, b1, miner2)
	if err := node.pow.Chain.ApplyBlock(b2); err != nil {
		t.Fatalf("apply competing b2: %v", err)
	}

	// removed-before-added: first the orphaned emit rolls back, then the
	// branch's off-tip re-emit replays
	if removed, topic := readSub(); !removed || topic != "key" {
		t.Fatalf("rollback notification = {removed:%v topic:%q}, want {true key}", removed, topic)
	}
	if removed, topic := readSub(); removed || topic != "key" {
		t.Fatalf("replay notification = {removed:%v topic:%q}, want {false key}", removed, topic)
	}
}
