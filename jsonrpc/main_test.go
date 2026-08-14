// Package jsonrpc_test drives every rpc method against a real
// in-process full node: the server listens on an ephemeral port, and a
// plain http client sends jsonrpc2 messages exactly like a wallet or
// miner would.
package jsonrpc_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/mr-tron/base58"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngpool"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

type rpcNode struct {
	pow *consensus.PoWork
	url string
}

// newRPCNode boots a full node (storage + chain + state + p2p +
// consensus) plus its rpc server on an ephemeral port
func newRPCNode(t *testing.T) *rpcNode {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "chain.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	storage.InitDB(db)

	store := ngblocks.Init(db, ngtypes.ZERONET)
	state := ngstate.InitStateFromGenesis(db, ngtypes.ZERONET)
	chain := blockchain.Init(db, ngtypes.ZERONET, store, state)
	chain.CheckHealth(ngtypes.ZERONET)

	local := ngp2p.InitLocalNode(chain, ngp2p.P2PConfig{
		P2PKeyFile:                  filepath.Join(t.TempDir(), "p2p.key"),
		Network:                     ngtypes.ZERONET,
		Port:                        0, // ephemeral
		DisableDiscovery:            true,
		DisableConnectingBootstraps: true,
	})
	local.GoServe()

	pool := ngpool.Init(db, chain, local)

	pow := consensus.InitPoWConsensus(db, chain, pool, state, local, consensus.PoWorkConfig{
		Network:                     ngtypes.ZERONET,
		DisableConnectingBootstraps: true,
	})
	pow.GoLoop()

	// bind the listener first, so the port can never collide
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	rpc := jsonrpc.NewServer(pow, jsonrpc.ServerConfig{Host: "127.0.0.1"})
	go func() { _ = rpc.Server.Server.Serve(listener) }()

	t.Cleanup(func() {
		_ = rpc.Server.Server.Close()
		pow.Stop()
		time.Sleep(100 * time.Millisecond)
		_ = db.Close()
	})

	return &rpcNode{
		pow: pow,
		url: "http://" + listener.Addr().String(),
	}
}

// call posts one jsonrpc2 request and returns the raw result or error
func (n *rpcNode) call(t *testing.T, method string, params any) (json.RawMessage, *jsonrpc2.Error) {
	t.Helper()

	rawParams, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(jsonrpc2.NewJsonRpcRequest(1, method, rawParams))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Post(n.url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	reply, err := jsonrpc2.UnmarshalMessage(raw)
	if err != nil {
		t.Fatalf("%s replied invalid jsonrpc: %v: %s", method, err, raw)
	}

	if reply.Error != nil {
		return nil, reply.Error
	}
	if reply.Result == nil {
		return nil, nil
	}
	return json.RawMessage(*reply.Result), nil
}

// mustCall fails the test on any rpc-level error
func (n *rpcNode) mustCall(t *testing.T, method string, params any) json.RawMessage {
	t.Helper()

	result, rpcErr := n.call(t, method, params)
	if rpcErr != nil {
		t.Fatalf("%s: %+v", method, rpcErr)
	}
	return result
}

// decodeInto unmarshals a result into out, failing the test on error
func decodeInto(t *testing.T, raw json.RawMessage, out any) {
	t.Helper()

	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("cannot decode result %s: %v", raw, err)
	}
}

func bs58Key(key *ngtypes.PrivateKey) string {
	return base58.FastBase58Encoding(key.Serialize())
}

// mineViaRPC runs the real miner loop over rpc: getWork, seal the
// template locally (ZERONET difficulty is 1) and submitWork
func mineViaRPC(t *testing.T, node *rpcNode, miner *ngtypes.PrivateKey) {
	t.Helper()

	var work struct {
		WorkID uint64 `json:"id"`
		Block  string `json:"block"`
		Txs    string `json:"txs"`
	}
	decodeInto(t, node.mustCall(t, "getWork", nil), &work)

	var block ngtypes.FullBlock
	if err := utils.HexRLPDecode(work.Block, &block); err != nil {
		t.Fatal(err)
	}
	var txs []*ngtypes.FullTx
	if err := utils.HexRLPDecode(work.Txs, &txs); err != nil {
		t.Fatal(err)
	}

	height := block.GetHeight()
	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, height,
		[]ngtypes.Address{ngtypes.NewAddress(miner)},
		[]*big.Int{ngtypes.GetBlockReward(height)},
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}

	if err := block.ToUnsealing(append([]*ngtypes.FullTx{genTx}, txs...)); err != nil {
		t.Fatal(err)
	}

	for n := uint64(0); n < 1_000_000; n++ {
		nonce := utils.PackUint64LE(n)
		if err := block.ToSealed(nonce); err != nil {
			t.Fatal(err)
		}
		if block.CheckError() == nil {
			node.mustCall(t, "submitWork", map[string]any{
				"id":    work.WorkID,
				"nonce": hex.EncodeToString(nonce),
				"gen":   utils.HexRLPEncode(genTx),
			})
			return
		}
	}

	t.Fatal("failed to seal the rpc block template")
}

func TestRPCPing(t *testing.T) {
	node := newRPCNode(t)

	var pong string
	decodeInto(t, node.mustCall(t, "ping", nil), &pong)
	if pong != "pong" {
		t.Fatalf("ping = %q, want pong", pong)
	}
}

// TestRPCChainQueries covers the read-only chain methods against the
// freshly initialized genesis chain
func TestRPCChainQueries(t *testing.T) {
	node := newRPCNode(t)
	genesisHash := node.pow.Chain.GetLatestBlockHash()

	var network string
	decodeInto(t, node.mustCall(t, "getNetwork", nil), &network)
	if network != ngtypes.ZERONET.String() {
		t.Fatalf("getNetwork = %q, want %q", network, ngtypes.ZERONET.String())
	}

	var height uint64
	decodeInto(t, node.mustCall(t, "getLatestBlockHeight", nil), &height)
	if height != 0 {
		t.Fatalf("getLatestBlockHeight = %d, want 0", height)
	}

	var hash []byte
	decodeInto(t, node.mustCall(t, "getLatestBlockHash", nil), &hash)
	if !bytes.Equal(hash, genesisHash) {
		t.Fatalf("getLatestBlockHash = %x, want %x", hash, genesisHash)
	}

	var latest ngtypes.FullBlock
	decodeInto(t, node.mustCall(t, "getLatestBlock", nil), &latest)
	if latest.GetHeight() != 0 {
		t.Fatalf("getLatestBlock height = %d, want 0", latest.GetHeight())
	}

	var byHeight ngtypes.FullBlock
	decodeInto(t, node.mustCall(t, "getBlockByHeight", map[string]any{"height": 0}), &byHeight)
	if !bytes.Equal(byHeight.GetHash(), genesisHash) {
		t.Fatal("getBlockByHeight(0) is not the genesis block")
	}

	var byHash ngtypes.FullBlock
	decodeInto(t, node.mustCall(t, "getBlockByHash",
		map[string]any{"hash": hex.EncodeToString(genesisHash)}), &byHash)
	if byHash.GetHeight() != 0 {
		t.Fatal("getBlockByHash(genesis) is not the genesis block")
	}

	if _, rpcErr := node.call(t, "getBlockByHeight", map[string]any{"height": 999}); rpcErr == nil {
		t.Fatal("getBlockByHeight(999) should fail on an empty chain")
	}

	if _, rpcErr := node.call(t, "getPeers", nil); rpcErr != nil {
		t.Fatalf("getPeers: %+v", rpcErr)
	}
}

// TestRPCAccountQueries covers balance and slot lookups by address
func TestRPCAccountQueries(t *testing.T) {
	node := newRPCNode(t)

	// balance by address works for any funded address
	miner, _ := ngtypes.GenerateKey()
	mineViaRPC(t, node, miner)

	var balance struct {
		TotalBalance  string
		MatureBalance string
		LockedBalance string
	}
	decodeInto(t, node.mustCall(t, "getBalanceByAddress",
		map[string]any{"address": ngtypes.NewAddress(miner).BS58()}), &balance)
	if want := ngtypes.GetBlockReward(1).String(); balance.TotalBalance != want {
		t.Fatalf("getBalanceByAddress = %s, want %s", balance.TotalBalance, want)
	}

	// an address without a contract slot has no account entry
	if _, rpcErr := node.call(t, "getAccountByAddress",
		map[string]any{"address": ngtypes.NewAddress(miner).BS58()}); rpcErr == nil {
		t.Fatal("getAccountByAddress before deploying should fail")
	}
}

// TestRPCUtils covers the wallet helper methods
func TestRPCUtils(t *testing.T) {
	node := newRPCNode(t)

	key, _ := ngtypes.GenerateKey()

	var reply struct {
		Address ngtypes.Address
	}
	decodeInto(t, node.mustCall(t, "publicKeyToAddress",
		map[string]any{"PrivateKeys": []string{bs58Key(key)}}), &reply)

	if want := ngtypes.NewAddress(key); !reply.Address.Equals(want) {
		t.Fatalf("publicKeyToAddress = %s, want %s", reply.Address, want)
	}
}

// TestRPCMiningLoop drives the external-miner protocol: getWork hands
// out a sealable template and submitWork lands the block on chain with
// the reward credited
func TestRPCMiningLoop(t *testing.T) {
	node := newRPCNode(t)
	miner, _ := ngtypes.GenerateKey()

	mineViaRPC(t, node, miner)

	var height uint64
	decodeInto(t, node.mustCall(t, "getLatestBlockHeight", nil), &height)
	if height != 1 {
		t.Fatalf("height after submitWork = %d, want 1", height)
	}

	var balance struct{ TotalBalance string }
	decodeInto(t, node.mustCall(t, "getBalanceByAddress",
		map[string]any{"address": ngtypes.NewAddress(miner).BS58()}), &balance)
	if want := ngtypes.GetBlockReward(1).String(); balance.TotalBalance != want {
		t.Fatalf("miner balance = %s, want %s", balance.TotalBalance, want)
	}

	// a stale/unknown work id must be rejected
	if _, rpcErr := node.call(t, "submitWork", map[string]any{
		"id": 42, "nonce": "00", "gen": "00",
	}); rpcErr == nil {
		t.Fatal("submitWork with an unknown work id should fail")
	}
}

// TestRPCContractLifecycle walks a contract through its whole life
// purely over rpc: fund -> deploy (first edit = namespace purchase) ->
// activate (lock) -> dry-run -> trigger -> receipt
func TestRPCContractLifecycle(t *testing.T) {
	const contractWat = `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (import "log" "emit" (func $emit (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))
    (drop (call $emit (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))))
`
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	// signAndSend takes an unsigned tx from a gen* method, signs it with
	// the rpc signer and broadcasts it; returns the tx hash
	signAndSend := func(unsignedHex string) string {
		t.Helper()

		var signedHex string
		decodeInto(t, node.mustCall(t, "signTx", map[string]any{
			"rawTx":       unsignedHex,
			"privateKeys": []string{bs58Key(key)},
		}), &signedHex)

		var txHash string
		decodeInto(t, node.mustCall(t, "sendTx", map[string]any{"rawTx": signedHex}), &txHash)
		return txHash
	}

	genResult := func(method string, params any) string {
		t.Helper()

		var rawHex string
		decodeInto(t, node.mustCall(t, method, params), &rawHex)
		return rawHex
	}

	// fund the deployer: two block rewards cover the deploy fee + change
	mineViaRPC(t, node, key)
	mineViaRPC(t, node, key)

	// deploy: the FIRST edit opens the address's contract slot (the
	// namespace purchase — DeployFee burned on top of the tx fee)
	signAndSend(genResult("genCommit", map[string]any{
		"address": addr.BS58(),
		"fee":     0.05,
		"hunks":   []map[string]any{{"pos": 0, "del": "", "ins": contractWat}},
	}))
	mineViaRPC(t, node, key)

	var account struct {
		Owner string `json:"owner"`
	}
	decodeInto(t, node.mustCall(t, "getAccountByAddress",
		map[string]any{"address": addr.BS58()}), &account)
	if account.Owner != addr.BS58() {
		t.Fatalf("slot owner = %s, want %s", account.Owner, addr.BS58())
	}

	var text string
	decodeInto(t, node.mustCall(t, "getContract", map[string]any{"address": addr.BS58()}), &text)
	if text != contractWat {
		t.Fatal("getContract mismatch after the commit tx")
	}

	// activate: the sender locks its own slot
	signAndSend(genResult("genActivate", map[string]any{"fee": 0.05}))
	mineViaRPC(t, node, key)

	// dry-run by address: nothing lands on chain, but the simulated
	// run reports success, gas and events
	var dryRun struct {
		Success bool   `json:"success"`
		GasUsed uint64 `json:"gasUsed"`
		Events  []struct {
			Contract string `json:"contract"`
			Topic    string `json:"topic"`
		} `json:"events"`
	}
	decodeInto(t, node.mustCall(t, "callContract", map[string]any{
		"contract": addr.BS58(),
	}), &dryRun)
	if !dryRun.Success {
		t.Fatal("callContract dry-run failed")
	}
	if dryRun.GasUsed < 1000 {
		t.Fatalf("dry-run gasUsed = %d: the kv.set tier is missing", dryRun.GasUsed)
	}
	if len(dryRun.Events) != 1 || dryRun.Events[0].Topic != "key" || dryRun.Events[0].Contract != addr.BS58() {
		t.Fatalf("dry-run events = %+v", dryRun.Events)
	}

	acc, err := node.pow.State.GetAccountByAddress(addr)
	if err != nil {
		t.Fatal(err)
	}
	if len(acc.Context.Get("key")) != 0 {
		t.Fatal("the dry-run leaked state onto the chain")
	}

	// trigger for real: a transact tx to the contract address runs main
	txHash := signAndSend(genResult("genTransaction", map[string]any{
		"participants": []string{addr.BS58()},
		"values":       []float64{0},
		"fee":          0.01,
		"extra":        "",
	}))
	mineViaRPC(t, node, key)

	acc, err = node.pow.State.GetAccountByAddress(addr)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(acc.Context.Get("key")); got != "val" {
		t.Fatalf("contract kv = %q, want val", got)
	}

	// the tx is now on chain with a receipt recording the run
	var txReply struct {
		OnChain       bool   `json:"onChain"`
		Confirmations uint64 `json:"confirmations"`
	}
	decodeInto(t, node.mustCall(t, "getTxByHash", map[string]any{"hash": txHash}), &txReply)
	if !txReply.OnChain || txReply.Confirmations < 1 {
		t.Fatalf("getTxByHash = %+v, want on-chain with confirmations", txReply)
	}

	var receipt struct {
		OnChain bool `json:"onChain"`
		Runs    []struct {
			Contract string `json:"contract"`
			Success  bool   `json:"success"`
			GasUsed  uint64 `json:"gasUsed"`
			Events   []struct {
				Topic string `json:"topic"`
			} `json:"events"`
		} `json:"runs"`
	}
	decodeInto(t, node.mustCall(t, "getReceipt", map[string]any{"hash": txHash}), &receipt)
	if !receipt.OnChain {
		t.Fatal("getReceipt: tx should be on chain")
	}
	if len(receipt.Runs) != 1 || !receipt.Runs[0].Success || receipt.Runs[0].Contract != addr.BS58() {
		t.Fatalf("getReceipt runs = %+v", receipt.Runs)
	}
	if receipt.Runs[0].GasUsed == 0 || len(receipt.Runs[0].Events) != 1 ||
		receipt.Runs[0].Events[0].Topic != "key" {
		t.Fatalf("getReceipt run detail = %+v", receipt.Runs[0])
	}
}
