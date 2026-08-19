package jsonrpc_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// TestRPCSubmitWorkRejections drives every failure seam of the
// getWork/submitWork protocol with a REAL work template: bad hex, a
// non-generate gen tx, a short nonce and a nonce that fails the target
func TestRPCSubmitWorkRejections(t *testing.T) {
	node := newRPCNode(t)
	miner, _ := ngtypes.GenerateKey()

	var work struct {
		WorkID uint64 `json:"id"`
		Block  string `json:"block"`
		Txs    string `json:"txs"`
	}
	decodeInto(t, node.mustCall(t, "getWork", nil), &work)

	// rebuild the template locally, exactly like a miner would
	var block ngtypes.FullBlock
	if err := utils.HexRLPDecode(work.Block, &block); err != nil {
		t.Fatal(err)
	}

	height := block.GetHeight()
	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		ngtypes.GetBlockReward(height),
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	genHex := utils.HexRLPEncode(genTx)

	// a gen tx claiming MORE than the block reward: it survives the
	// unsealing shape checks but the sealed block must fail verification
	greedyTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		new(big.Int).Add(ngtypes.GetBlockReward(height), big.NewInt(1)),
		big.NewInt(0), nil, nil)
	if err := greedyTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	greedyHex := utils.HexRLPEncode(greedyTx)

	fullNonce := hex.EncodeToString(utils.PackUint64LE(0)) // correct length

	// a non-generate tx in the gen slot: the block cannot unseal
	var notGenHex string
	decodeInto(t, node.mustCall(t, "genActivate", map[string]any{"fee": "0"}), &notGenHex)

	for _, c := range []struct {
		name   string
		params map[string]any
	}{
		{"unknownWork", map[string]any{"id": 0, "nonce": "00", "gen": genHex}},
		{"badNonceHex", map[string]any{"id": work.WorkID, "nonce": "zz", "gen": genHex}},
		{"badGenHex", map[string]any{"id": work.WorkID, "nonce": "00", "gen": "zz"}},
		{"notGenerateTx", map[string]any{"id": work.WorkID, "nonce": "00", "gen": notGenHex}},
		{"shortNonce", map[string]any{"id": work.WorkID, "nonce": "00", "gen": genHex}},
		{"greedyReward", map[string]any{"id": work.WorkID, "nonce": fullNonce, "gen": greedyHex}},
	} {
		if _, rpcErr := node.call(t, "submitWork", c.params); rpcErr == nil {
			t.Errorf("submitWork/%s: accepted, want a jsonrpc error", c.name)
		}
	}

	// the same work id must still be minable after all the rejections
	mineViaRPC(t, node, miner)
	var heightNow uint64
	decodeInto(t, node.mustCall(t, "getLatestBlockHeight", nil), &heightNow)
	if heightNow != 1 {
		t.Fatalf("height after rejected submissions + a real mine = %d, want 1", heightNow)
	}
}
