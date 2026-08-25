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
	return sealBlockOnAll(t, parent, miner, nil, extra...)
}

// sealBlockOnAll is sealBlockOn with blind commitments packed in too. The
// commitments must be attached before ToUnsealing, which folds them into the
// content root alongside the txs.
func sealBlockOnAll(t *testing.T, parent *ngtypes.FullBlock, miner *ngtypes.PrivateKey, commits []*ngtypes.Commitment, extra ...*ngtypes.FullTx) *ngtypes.FullBlock {
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
	if len(commits) != 0 {
		block.SetCommits(commits)
	}
	if err := block.ToUnsealing(append([]*ngtypes.FullTx{genTx}, extra...)); err != nil {
		t.Fatal(err)
	}
	// ToUnsealing computes the witness root over insertion order (gen first) but
	// stores x.Txs hash-sorted; recompute over the canonical sorted order so a
	// block whose packed reveal sorts ahead of the generate is not rejected as
	// witness-invalid.
	block.BlockHeader.WitnessRoot = ngtypes.CalcWitnessRoot(block.Txs, block.Commits)

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
  (func (export "ng:main")
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

	gen := func(method string, params any) string {
		var unsigned string
		decodeInto(t, node.mustCall(t, method, params), &unsigned)
		return unsigned
	}
	mineViaRPC(t, node, key) // fund
	commitReveal(t, node, key, gen("ng_genCommit", map[string]any{"fee": "0.05", "wasm": hex.EncodeToString(mustWat(contractWat))}))
	mineViaRPC(t, node, key) // deploy goes live at once

	// the tip after deployment is the fork point. The transact reveal must be
	// blindly committed one block BELOW its own height, so both the canonical
	// chain and the competing branch grow that commit block themselves off the
	// fork point: a reorg only restores a consumed commitment by re-applying its
	// recording block, so the commit has to live ABOVE the fork point on each
	// branch (not be a shared ancestor).
	forkParent, ok := node.pow.Chain.GetLatestBlock().(*ngtypes.FullBlock)
	if !ok {
		t.Fatal("latest block is not a *FullBlock")
	}
	commitHeight := forkParent.GetHeight() + 1
	revealHeight := commitHeight + 1

	// build the transact reveal (locked on revealHeight) and keep a copy: the
	// canonical chain reveals it now, the competing branch reuses it (same
	// height, same fork-point state) to re-emit below its new tip
	unsigned := gen("ng_genTransaction", map[string]any{"to": addr.BS58(), "value": "0", "fee": "0.01"})
	rawUnsigned, err := hex.DecodeString(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	var transactTx ngtypes.FullTx
	if err := rlp.DecodeBytes(rawUnsigned, &transactTx); err != nil {
		t.Fatal(err)
	}
	transactTx.Height = revealHeight
	transactTx.Salt = rpcTestSalt
	if err := transactTx.Signature(key); err != nil {
		t.Fatal(err)
	}

	// the blind commitment over the reveal's salted preimage, recorded at
	// commitHeight (strictly below the reveal). Both branches pack this same
	// commitment into their own commit block.
	buf := append(append([]byte{}, transactTx.UnheightedHash()...), transactTx.Salt...)
	commit := ngtypes.NewCommitment(ngtypes.ZERONET, commitHeight, utils.Hash256(buf), big.NewInt(100_000_000_000_000))
	if err := commit.Signature(key); err != nil {
		t.Fatal(err)
	}
	commitRaw, err := rlp.EncodeToBytes(commit)
	if err != nil {
		t.Fatal(err)
	}
	node.mustCall(t, "ng_sendCommitment", map[string]any{"rawCommitment": hex.EncodeToString(commitRaw)})
	mineViaRPC(t, node, key) // canonical commit block @commitHeight

	signedTx, err := rlp.EncodeToBytes(&transactTx)
	if err != nil {
		t.Fatal(err)
	}
	node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": hex.EncodeToString(signedTx)})

	mineViaRPC(t, node, key) // canonical reveal block @revealHeight -> emits the "key" log

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
	// its blocks never collide with the canonical ones: b0 re-records the same
	// blind commitment, b1 carries the same transact reveal (so it re-emits below
	// the new tip), b2 is the empty tip that out-works the canonical two-block
	// commit+reveal, forcing the reorg
	miner2, _ := ngtypes.GenerateKey()
	b0 := sealBlockOnAll(t, forkParent, miner2, []*ngtypes.Commitment{commit})
	if err := node.pow.Chain.ApplyBlock(b0); err != nil {
		t.Fatalf("apply competing b0: %v", err)
	}
	b1 := sealBlockOn(t, b0, miner2, &transactTx)
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
