package jsonrpc_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestRPCForkDataSources drives the three fork-chain data sources the
// way `ngcore fork --rpc` consumes them: getHead for the boot info,
// getAddressState for lazy per-address pulls and getSheet for the eager
// one-shot export. The hex(RLP) payloads must decode back into the
// exact on-chain state.
func TestRPCForkDataSources(t *testing.T) {
	node := newRPCNode(t)

	// a NON-recovery scheme key: its full envelope registers the public
	// key on chain, so the sheet's key registry has content (secp256k1
	// recover envelopes never touch the registry)
	key, err := ngtypes.GenerateSchemeKey(ngtypes.SchemeMLDSA44)
	if err != nil {
		t.Fatal(err)
	}
	addr := ngtypes.NewAddress(key)

	// fund the address, then deploy a contract so every data source has
	// real content: a balance, a contract slot and a registered key
	mineViaRPC(t, node, key)

	contractWasm := mustWat(`(module (func (export "main")))`)

	var unsignedHex string
	decodeInto(t, node.mustCall(t, "ng_genCommit", map[string]any{
		"fee":  "0.05",
		"wasm": hex.EncodeToString(contractWasm),
	}), &unsignedHex)

	commitReveal(t, node, key, unsignedHex)
	mineViaRPC(t, node, key)

	latest := node.pow.Chain.GetLatestBlock()

	// --- getHead: the light fork-boot info ---
	var head struct {
		Network   uint8  `json:"network"`
		Height    uint64 `json:"height"`
		BlockHash string `json:"blockHash"`
		Timestamp uint64 `json:"timestamp"`
	}
	decodeInto(t, node.mustCall(t, "ng_getHead", nil), &head)

	if head.Network != uint8(ngtypes.ZERONET) {
		t.Fatalf("getHead network = %d, want %d", head.Network, uint8(ngtypes.ZERONET))
	}
	if head.Height != latest.GetHeight() {
		t.Fatalf("getHead height = %d, want %d", head.Height, latest.GetHeight())
	}
	if head.BlockHash != hex.EncodeToString(latest.GetHash()) {
		t.Fatalf("getHead blockHash = %s, want %x", head.BlockHash, latest.GetHash())
	}
	if head.Timestamp != latest.GetTimestamp() {
		t.Fatalf("getHead timestamp = %d, want %d", head.Timestamp, latest.GetTimestamp())
	}

	// --- getAddressState: one address's full state, the lazy-fork unit ---
	wantBalance, balErr := node.pow.State.GetTotalBalanceByAddress(addr)
	if balErr != nil {
		t.Fatal(balErr)
	}

	var state struct {
		Exists   bool   `json:"exists"`
		Balance  string `json:"balance"`
		Contract string `json:"contract"`
	}
	decodeInto(t, node.mustCall(t, "ng_getAddressState",
		map[string]any{"address": addr.BS58()}), &state)

	if !state.Exists {
		t.Fatal("getAddressState: a funded deployer must exist")
	}
	if state.Balance != wantBalance.String() {
		t.Fatalf("getAddressState balance = %s, want %s", state.Balance, wantBalance)
	}
	if state.Contract == "" {
		t.Fatal("getAddressState: the contract slot is missing")
	}

	// the contract field is hex(rlp(ngtypes.Contract)): it must decode
	// back into the deployed slot exactly
	rawContract, err := hex.DecodeString(state.Contract)
	if err != nil {
		t.Fatalf("getAddressState contract is not hex: %v", err)
	}
	var contract ngtypes.Contract
	if err := rlp.DecodeBytes(rawContract, &contract); err != nil {
		t.Fatalf("getAddressState contract does not RLP-decode: %v", err)
	}
	if !contract.Owner.Equals(addr) {
		t.Fatalf("decoded contract owner = %s, want %s", contract.Owner, addr)
	}
	if !bytes.Equal(contract.Source, contractWasm) {
		t.Fatal("decoded contract source differs from the deployed wasm")
	}

	// an untouched address: no balance, no slot, exists=false
	other, _ := ngtypes.GenerateKey()
	var empty struct {
		Exists   bool   `json:"exists"`
		Balance  string `json:"balance"`
		Contract string `json:"contract"`
	}
	decodeInto(t, node.mustCall(t, "ng_getAddressState",
		map[string]any{"address": ngtypes.NewAddress(other).BS58()}), &empty)
	if empty.Exists || empty.Balance != "0" || empty.Contract != "" {
		t.Fatalf("getAddressState(untouched) = %+v, want empty", empty)
	}

	// rejected inputs
	if _, rpcErr := node.call(t, "ng_getAddressState",
		map[string]any{"address": "not-base58-0OIl"}); rpcErr == nil {
		t.Fatal("getAddressState must reject a malformed address")
	}
	if _, rpcErr := node.call(t, "ng_getAddressState", []int{1, 2}); rpcErr == nil {
		t.Fatal("getAddressState must reject non-object params")
	}

	// --- getSheet: the eager whole-state export ---
	var sheetReply struct {
		Network   uint8  `json:"network"`
		Height    uint64 `json:"height"`
		BlockHash string `json:"blockHash"`
		Timestamp uint64 `json:"timestamp"`
		Sheet     string `json:"sheet"`
	}
	decodeInto(t, node.mustCall(t, "ng_getSheet", nil), &sheetReply)

	if sheetReply.Network != uint8(ngtypes.ZERONET) {
		t.Fatalf("getSheet network = %d, want %d", sheetReply.Network, uint8(ngtypes.ZERONET))
	}
	if sheetReply.Height != latest.GetHeight() {
		t.Fatalf("getSheet height = %d, want %d", sheetReply.Height, latest.GetHeight())
	}
	if sheetReply.BlockHash != hex.EncodeToString(latest.GetHash()) {
		t.Fatalf("getSheet blockHash = %s, want %x", sheetReply.BlockHash, latest.GetHash())
	}
	if sheetReply.Timestamp != latest.GetTimestamp() {
		t.Fatalf("getSheet timestamp = %d, want %d", sheetReply.Timestamp, latest.GetTimestamp())
	}

	rawSheet, err := hex.DecodeString(sheetReply.Sheet)
	if err != nil {
		t.Fatalf("getSheet sheet is not hex: %v", err)
	}
	var sheet ngtypes.Sheet
	if err := rlp.DecodeBytes(rawSheet, &sheet); err != nil {
		t.Fatalf("getSheet sheet does not RLP-decode: %v", err)
	}

	if sheet.Network != ngtypes.ZERONET || sheet.Height != latest.GetHeight() ||
		!bytes.Equal(sheet.BlockHash, latest.GetHash()) {
		t.Fatalf("decoded sheet head = %d/%d/%x, want %d/%d/%x",
			sheet.Network, sheet.Height, sheet.BlockHash,
			ngtypes.ZERONET, latest.GetHeight(), latest.GetHash())
	}

	var foundBalance, foundContract, foundKey bool
	for _, b := range sheet.Balances {
		if b.Address.Equals(addr) {
			foundBalance = true
			if b.Amount.Cmp(wantBalance) != 0 {
				t.Fatalf("sheet balance = %s, want %s", b.Amount, wantBalance)
			}
		}
	}
	for _, c := range sheet.Contracts {
		if c.Owner.Equals(addr) {
			foundContract = true
			if !bytes.Equal(c.Source, contractWasm) {
				t.Fatal("sheet contract source differs from the deployed wasm")
			}
		}
	}
	for _, k := range sheet.Keys {
		if k.Address.Equals(addr) {
			foundKey = true
		}
	}
	if !foundBalance || !foundContract || !foundKey {
		t.Fatalf("sheet misses the deployer: balance=%v contract=%v key=%v",
			foundBalance, foundContract, foundKey)
	}
}
