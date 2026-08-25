package jsonrpc_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// TestRPCGenDeploy pins the unsigned-tx composer that has no other
// coverage: the raw hex(RLP) reply must decode back into a tx of the
// right type, height, fee and extra. An empty wasm is the DESTROY form —
// a deploy carrying the empty-code commit
func TestRPCGenDeploy(t *testing.T) {
	node := newRPCNode(t)
	nextHeight := node.pow.Chain.GetLatestBlockHeight() + 1

	// an empty wasm composes the destroy: a deploy whose Extra is the
	// empty-code commit
	var destroyHex string
	decodeInto(t, node.mustCall(t, "ng_genDeploy", map[string]any{
		"fee":  "0.01",
		"wasm": "",
	}), &destroyHex)

	var destroy ngtypes.FullTx
	if err := utils.HexRLPDecode(destroyHex, &destroy); err != nil {
		t.Fatalf("genDeploy reply does not RLP-decode: %v", err)
	}
	if destroy.Type != ngtypes.DeployTx {
		t.Fatalf("genDeploy type = %d, want DeployTx", destroy.Type)
	}
	if destroy.Height != nextHeight {
		t.Fatalf("genDeploy height = %d, want %d", destroy.Height, nextHeight)
	}
	// fee "0.01" NG = 10^16 raw units, exactly
	if want := new(big.Int).Exp(big.NewInt(10), big.NewInt(16), nil); destroy.Fee.Cmp(want) != 0 {
		t.Fatalf("genDeploy fee = %s, want %s", destroy.Fee, want)
	}
	// the empty-code commit decodes back to an empty module
	if code, err := ngtypes.DecodeCommitCode(destroy.Extra); err != nil || len(code) != 0 {
		t.Fatalf("genDeploy(empty wasm) extra = %x (code %x, err %v), want empty-code commit", destroy.Extra, code, err)
	}
	if destroy.IsSigned() {
		t.Fatal("genDeploy must return an UNSIGNED tx")
	}
}

// TestRPCParamRejections sweeps every method's input-validation seams:
// malformed params objects, bad hex, bad base58, bad amounts. Each call
// must come back as a jsonrpc error, never a success or a hang
func TestRPCParamRejections(t *testing.T) {
	node := newRPCNode(t)

	key, _ := ngtypes.GenerateKey()
	validAddr := ngtypes.NewAddress(key).BS58()

	// a structurally valid unsigned tx for the semantic-failure cases
	var unsignedHex string
	decodeInto(t, node.mustCall(t, "ng_genDeploy", map[string]any{"fee": "0", "wasm": ""}), &unsignedHex)

	shortKey := base58.FastBase58Encoding([]byte{1, 2, 3}) // valid bs58, invalid key

	for _, c := range []struct {
		name   string
		method string
		params any
	}{
		// non-object params fail the unmarshal step of every method
		{"getBlockByHeight/params", "ng_getBlockByHeight", []int{1}},
		{"getBlockByHash/params", "ng_getBlockByHash", []int{1}},
		{"getTxByHash/params", "ng_getTxByHash", []int{1}},
		{"sendTx/params", "ng_sendTx", []int{1}},
		{"genTransaction/params", "ng_genTransaction", []int{1}},
		{"genCommit/params", "ng_genCommit", []int{1}},
		{"genDeploy/params", "ng_genDeploy", []int{1}},
		{"callContract/params", "ng_callContract", []int{1}},
		{"getReceipt/params", "ng_getReceipt", []int{1}},
		{"getContractInfo/params", "ng_getContractInfo", []int{1}},
		{"getContractStorage/params", "ng_getContractStorage", []int{1}},
		{"getBalanceByAddress/params", "ng_getBalanceByAddress", []int{1}},
		{"publicKeyToAddress/params", "ng_publicKeyToAddress", []int{1}},
		{"addPeer/params", "admin_addPeer", []int{1}},
		{"submitWork/params", "ng_submitWork", []int{1}},

		// hex decoding
		{"getBlockByHash/hex", "ng_getBlockByHash", map[string]any{"hash": "zz"}},
		{"getTxByHash/hex", "ng_getTxByHash", map[string]any{"hash": "zz"}},
		{"getReceipt/hex", "ng_getReceipt", map[string]any{"hash": "zz"}},
		{"sendTx/hex", "ng_sendTx", map[string]any{"rawTx": "zz"}},
		{"getContractStorage/hex", "ng_getContractStorage",
			map[string]any{"address": validAddr, "key": "zz"}},
		{"genCommit/hex", "ng_genCommit", map[string]any{"fee": "0", "wasm": "zz"}},
		{"genDeploy/hex", "ng_genDeploy", map[string]any{"fee": "0", "wasm": "zz"}},
		{"genTransaction/hex", "ng_genTransaction",
			map[string]any{"to": validAddr, "value": "0", "fee": "0", "extra": "zz"}},

		// rlp decoding (valid hex, not a tx)
		{"sendTx/rlp", "ng_sendTx", map[string]any{"rawTx": "00"}},

		// base58 addresses
		{"getContractInfo/bs58", "ng_getContractInfo", map[string]any{"address": "0OIl"}},
		{"getContractStorage/bs58", "ng_getContractStorage",
			map[string]any{"address": "0OIl", "key": "00"}},
		{"getBalanceByAddress/bs58", "ng_getBalanceByAddress", map[string]any{"address": "0OIl"}},
		{"callContract/bs58", "ng_callContract", map[string]any{"contract": "0OIl"}},
		{"genTransaction/bs58", "ng_genTransaction",
			map[string]any{"to": "0OIl", "value": "0", "fee": "0"}},

		// NG amounts
		{"genTransaction/value", "ng_genTransaction",
			map[string]any{"to": validAddr, "value": "abc", "fee": "0"}},
		{"genTransaction/fee", "ng_genTransaction",
			map[string]any{"to": validAddr, "value": "0", "fee": "-1"}},
		{"genDeploy/fee", "ng_genDeploy", map[string]any{"fee": "abc", "wasm": ""}},
		{"genCommit/fee", "ng_genCommit", map[string]any{"fee": "abc", "wasm": ""}},
		{"callContract/value", "ng_callContract",
			map[string]any{"contract": validAddr, "value": "abc"}},

		// signer keys
		{"publicKeyToAddress/noKeys", "ng_publicKeyToAddress",
			map[string]any{"PrivateKeys": []string{}}},
		{"publicKeyToAddress/badBs58", "ng_publicKeyToAddress",
			map[string]any{"PrivateKeys": []string{"0OIl"}}},
		{"publicKeyToAddress/badKey", "ng_publicKeyToAddress",
			map[string]any{"PrivateKeys": []string{shortKey}}},

		// semantic failures
		{"sendTx/unsigned", "ng_sendTx", map[string]any{"rawTx": unsignedHex}},
		{"getBlockByHash/unknown", "ng_getBlockByHash",
			map[string]any{"hash": "00000000000000000000000000000000"}},
		{"getTxByHash/unknown", "ng_getTxByHash",
			map[string]any{"hash": "0000000000000000000000000000000000000000000000000000000000000000"}},
		{"getContractInfo/noSlot", "ng_getContractInfo", map[string]any{"address": validAddr}},
		{"getContractStorage/noSlot", "ng_getContractStorage",
			map[string]any{"address": validAddr, "key": "00"}},
		{"callContract/noSlot", "ng_callContract",
			map[string]any{"contract": validAddr, "value": "0"}},
	} {
		if _, rpcErr := node.call(t, c.method, c.params); rpcErr == nil {
			t.Errorf("%s: accepted %v, want a jsonrpc error", c.name, c.params)
		}
	}
}

// TestRPCTxInPool covers getTxByHash's mempool branch: a broadcast but
// not yet mined tx reports onChain=false, and getReceipt for a tx the
// chain never saw reports no runs
func TestRPCTxInPool(t *testing.T) {
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	mineViaRPC(t, node, key)

	var unsignedHex string
	decodeInto(t, node.mustCall(t, "ng_genTransaction", map[string]any{
		"to":    ngtypes.NewAddress(key).BS58(),
		"value": "1",
		"fee":   "0.01",
	}), &unsignedHex)

	// commitReveal lands the commitment and leaves the reveal pending
	txHash := commitReveal(t, node, key, unsignedHex)

	var reply struct {
		OnChain       bool            `json:"onChain"`
		Tx            *ngtypes.FullTx `json:"tx"`
		Confirmations uint64          `json:"confirmations"`
	}
	decodeInto(t, node.mustCall(t, "ng_getTxByHash", map[string]any{"hash": txHash}), &reply)
	if reply.OnChain {
		t.Fatal("an unmined tx must report onChain=false")
	}
	if reply.Tx == nil {
		t.Fatal("getTxByHash must return the pooled tx")
	}
	if reply.Confirmations != 0 {
		t.Fatalf("pooled tx confirmations = %d, want 0", reply.Confirmations)
	}

	// a receipt for a hash the chain never executed: not on chain, no runs
	var receipt struct {
		OnChain bool  `json:"onChain"`
		Runs    []any `json:"runs"`
	}
	decodeInto(t, node.mustCall(t, "ng_getReceipt", map[string]any{"hash": txHash}), &receipt)
	if receipt.OnChain || len(receipt.Runs) != 0 {
		t.Fatalf("getReceipt(pooled tx) = %+v, want off-chain with no runs", receipt)
	}
}

// TestRPCSyncGate pins the requireSynced wrapper: while the sync module
// is active every wrapped method must refuse to answer, and recover as
// soon as the sync ends
func TestRPCSyncGate(t *testing.T) {
	node := newRPCNode(t)

	node.pow.SyncMod.Lock()
	_, rpcErr := node.call(t, "ng_getLatestBlockHeight", nil)
	node.pow.SyncMod.Unlock()

	if rpcErr == nil {
		t.Fatal("a syncing node must refuse chain queries")
	}

	// once the sync is over the same method answers again
	node.mustCall(t, "ng_getLatestBlockHeight", nil)
}

// TestRPCCallContractFailure covers the dry-run failure path: a
// contract whose main traps must report success=false with the error
// message, without any rpc-level failure
func TestRPCCallContractFailure(t *testing.T) {
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	mineViaRPC(t, node, key)

	trapWasm := mustWat(`(module (func (export "ng:main") unreachable))`)

	var unsignedHex string
	decodeInto(t, node.mustCall(t, "ng_genCommit", map[string]any{
		"fee":  "0.05",
		"wasm": hex.EncodeToString(trapWasm),
	}), &unsignedHex)

	commitReveal(t, node, key, unsignedHex)
	mineViaRPC(t, node, key)

	var dryRun struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	decodeInto(t, node.mustCall(t, "ng_callContract", map[string]any{
		"contract": addr.BS58(),
	}), &dryRun)

	if dryRun.Ok {
		t.Fatal("a trapping contract must not dry-run successfully")
	}
	if dryRun.Error == "" {
		t.Fatal("the dry-run failure must carry the trap error")
	}
}
