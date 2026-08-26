package jsonrpc_test

import (
	"encoding/hex"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/statetrie"
)

// TestRPCStateRootAndProof mines a block via rpc, then checks ng_getStateRoot
// and ng_getStateProof: the proof round-trips through statetrie.Verify against
// the reported root (inclusion for the funded miner, absence for an untouched
// address), and a tampered proof is rejected.
func TestRPCStateRootAndProof(t *testing.T) {
	node := newRPCNode(t)
	miner, _ := ngtypes.GenerateKey()
	mineViaRPC(t, node, miner)

	var rootReply struct {
		Height    uint64 `json:"height"`
		StateRoot string `json:"stateRoot"`
	}
	decodeInto(t, node.mustCall(t, "ng_getStateRoot", nil), &rootReply)
	if rootReply.Height != 1 {
		t.Fatalf("state root height = %d, want 1", rootReply.Height)
	}
	rootHex := rootReply.StateRoot

	// the tip block header's StateRoot must equal the served root
	tip := node.pow.Chain.GetLatestBlock().(*ngtypes.FullBlock)
	if hex.EncodeToString(tip.BlockHeader.StateRoot) != rootHex {
		t.Fatalf("served root %s != tip header root %x", rootHex, tip.BlockHeader.StateRoot)
	}

	type proofReply struct {
		StateRoot string   `json:"stateRoot"`
		Path      string   `json:"path"`
		ValueHash string   `json:"valueHash"`
		Value     string   `json:"value"`
		Proof     []string `json:"proof"`
	}

	decodeProof := func(pr proofReply) (root, path, vh []byte, proof [][]byte) {
		root, _ = hex.DecodeString(pr.StateRoot)
		path, _ = hex.DecodeString(pr.Path)
		vh, _ = hex.DecodeString(pr.ValueHash)
		for _, p := range pr.Proof {
			b, _ := hex.DecodeString(p)
			proof = append(proof, b)
		}
		return
	}

	// inclusion: the miner's balance leaf
	var incl proofReply
	decodeInto(t, node.mustCall(t, "ng_getStateProof", map[string]any{
		"domain": "balance", "key": ngtypes.NewAddress(miner).BS58(),
	}), &incl)
	if incl.StateRoot != rootHex {
		t.Fatalf("proof root %s != state root %s", incl.StateRoot, rootHex)
	}
	root, path, vh, proof := decodeProof(incl)
	if !statetrie.Verify(root, path, vh, proof) {
		t.Fatal("inclusion proof rejected by statetrie.Verify")
	}
	if incl.Value == "" {
		t.Fatal("miner balance proof has an empty value")
	}
	// tamper: flip a sibling byte -> Verify must reject
	if len(proof) > 0 && len(proof[0]) > 0 {
		proof[0][0] ^= 0xff
		if statetrie.Verify(root, path, vh, proof) {
			t.Fatal("tampered proof accepted")
		}
	}

	// absence: an address that never received anything
	other, _ := ngtypes.GenerateKey()
	var abs proofReply
	decodeInto(t, node.mustCall(t, "ng_getStateProof", map[string]any{
		"domain": "balance", "key": ngtypes.NewAddress(other).BS58(),
	}), &abs)
	if abs.Value != "" {
		t.Fatalf("absence proof carries a value %q", abs.Value)
	}
	aroot, apath, avh, aproof := decodeProof(abs)
	if !statetrie.Verify(aroot, apath, avh, aproof) {
		t.Fatal("absence proof rejected by statetrie.Verify")
	}
}
