package jsonrpc_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// TestRPCSubmitWorkRejections drives every failure seam of the gen-aware
// getWork/submitWork protocol. getWork now folds the miner's generate in and
// seals the StateRoot, so a bad or non-generate gen fails at getWork; a greedy
// reward assembles fine but is rejected on submit (ApplyBlock re-checks the
// reward). submitWork itself only takes {id, nonce}.
func TestRPCSubmitWorkRejections(t *testing.T) {
	node := newRPCNode(t)
	miner, _ := ngtypes.GenerateKey()

	height := uint64(1) // next height on a fresh chain

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		ngtypes.GetBlockReward(height),
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	genHex := utils.HexRLPEncode(genTx)

	// a non-generate tx in the gen slot: AssembleWork's ToUnsealing rejects it
	var notGenHex string
	decodeInto(t, node.mustCall(t, "ng_genDeploy", map[string]any{"fee": "0", "wasm": ""}), &notGenHex)

	// --- getWork failure seams ---
	for _, c := range []struct {
		name   string
		params map[string]any
	}{
		{"badGenHex", map[string]any{"gen": "zz"}},
		{"notGenerateTx", map[string]any{"gen": notGenHex}},
	} {
		if _, rpcErr := node.call(t, "ng_getWork", c.params); rpcErr == nil {
			t.Errorf("getWork/%s: accepted, want a jsonrpc error", c.name)
		}
	}

	// a real work template to drive the submit-side rejections
	var work struct {
		WorkID uint64 `json:"id"`
		Block  string `json:"block"`
		Txs    string `json:"txs"`
	}
	decodeInto(t, node.mustCall(t, "ng_getWork", map[string]any{"gen": genHex}), &work)

	// --- submitWork failure seams ---
	for _, c := range []struct {
		name   string
		params map[string]any
	}{
		{"unknownWork", map[string]any{"id": 0, "nonce": "00"}},
		{"badNonceHex", map[string]any{"id": work.WorkID, "nonce": "zz"}},
		{"shortNonce", map[string]any{"id": work.WorkID, "nonce": "00"}},
	} {
		if _, rpcErr := node.call(t, "ng_submitWork", c.params); rpcErr == nil {
			t.Errorf("submitWork/%s: accepted, want a jsonrpc error", c.name)
		}
	}

	// a greedy reward assembles (blind mint) but is rejected on submit
	greedyTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		new(big.Int).Add(ngtypes.GetBlockReward(height), big.NewInt(1)),
		big.NewInt(0), nil, nil)
	if err := greedyTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	var greedyWork struct {
		WorkID uint64 `json:"id"`
	}
	decodeInto(t, node.mustCall(t, "ng_getWork",
		map[string]any{"gen": utils.HexRLPEncode(greedyTx)}), &greedyWork)
	if _, rpcErr := node.call(t, "ng_submitWork", map[string]any{
		"id": greedyWork.WorkID, "nonce": hex.EncodeToString(utils.PackUint64LE(0)),
	}); rpcErr == nil {
		t.Error("submitWork/greedyReward: accepted, want a jsonrpc error")
	}

	// mining still works after all the rejections
	mineViaRPC(t, node, miner)
	var heightNow uint64
	decodeInto(t, node.mustCall(t, "ng_getLatestBlockHeight", nil), &heightNow)
	if heightNow != 1 {
		t.Fatalf("height after rejected submissions + a real mine = %d, want 1", heightNow)
	}
}
