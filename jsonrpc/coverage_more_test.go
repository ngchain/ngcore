package jsonrpc_test

import (
	"encoding/hex"
	"net"
	"testing"
	"time"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// TestGetBalanceMatureError drives getBalanceByAddress's mature-balance
// error arm: dropping the block store's latest-height tag makes
// GetMatureBalanceByAddress fail, so the rpc must surface an error instead
// of a balance. (A DB-level fault injected from the test — production code
// is untouched.)
func TestGetBalanceMatureError(t *testing.T) {
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key).BS58()

	// the healthy node answers first
	node.mustCall(t, "ng_getBalanceByAddress", map[string]any{"address": addr})

	// remove the latest-height tag: GetMatureBalanceByAddress reads it and
	// now errors
	err := node.pow.Chain.Update(func(txn *bbolt.Tx) error {
		return txn.Bucket(storage.BlockBucketName).Delete(storage.LatestHeightTag)
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, rpcErr := node.call(t, "ng_getBalanceByAddress",
		map[string]any{"address": addr}); rpcErr == nil {
		t.Fatal("getBalanceByAddress must fail once the mature balance cannot resolve")
	}
}

// TestServe starts the blocking Serve() entry point on an ephemeral port
// and confirms it actually listens, then shuts it down. This is the
// production start path NewServer + Serve
func TestServe(t *testing.T) {
	node := newRPCNode(t)

	// bind an ephemeral port ourselves, then hand Serve a server whose
	// address is that port; Serve is blocking, so it runs in a goroutine
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}

	rpc := jsonrpc.NewServer(node.pow, jsonrpc.ServerConfig{Host: "127.0.0.1"})
	rpc.Server.Server.Addr = "127.0.0.1:" + portStr

	go rpc.Serve()

	// wait for the listener to come up
	var conn net.Conn
	for i := 0; i < 100; i++ {
		conn, err = net.Dial("tcp", "127.0.0.1:"+portStr)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if conn == nil {
		t.Fatalf("Serve did not start listening: %v", err)
	}
	_ = conn.Close()
	// Serve() panics on any ListenAndServe error (incl. ErrServerClosed),
	// so we intentionally leave it blocking: the goroutine is reaped when
	// the test binary exits, never closed, so it never panics
}

// TestServePanicsOnBindError covers Serve()'s failure arm: when
// ListenAndServe cannot bind (the port is already taken), Serve panics,
// which we recover here
func TestServePanicsOnBindError(t *testing.T) {
	node := newRPCNode(t)

	// hold a port so the server's bind is guaranteed to fail
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	rpc := jsonrpc.NewServer(node.pow, jsonrpc.ServerConfig{Host: "127.0.0.1"})
	rpc.Server.Server.Addr = held.Addr().String()

	done := make(chan struct{})
	go func() {
		defer func() {
			if recover() == nil {
				t.Errorf("Serve must panic when the bind fails")
			}
			close(done)
		}()
		rpc.Serve()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not fail on the bind conflict")
	}
}

// TestCallContractNoEvents dry-runs a contract whose main emits nothing:
// the success path still returns, with eventsToJSON taking its empty-slice
// branch (no events => nil)
func TestCallContractNoEvents(t *testing.T) {
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	mineViaRPC(t, node, key)

	// a contract that runs but emits no events
	quietWasm := mustWat(`(module (func (export "main")))`)

	signAndMine := func(unsignedHex string) {
		t.Helper()
		signedHex := localSign(t, key, unsignedHex)
		node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": signedHex})
		mineViaRPC(t, node, key)
	}

	var commitHex string
	decodeInto(t, node.mustCall(t, "ng_genCommit", map[string]any{
		"fee": "0.05", "wasm": hex.EncodeToString(quietWasm),
	}), &commitHex)
	signAndMine(commitHex)

	var activateHex string
	decodeInto(t, node.mustCall(t, "ng_genActivate", map[string]any{"fee": "0.05"}), &activateHex)
	signAndMine(activateHex)

	var dryRun struct {
		Ok     bool `json:"ok"`
		Events []struct {
			Topic string `json:"topic"`
		} `json:"events"`
	}
	decodeInto(t, node.mustCall(t, "ng_callContract", map[string]any{"contract": addr.BS58()}), &dryRun)
	if !dryRun.Ok {
		t.Fatal("a quiet contract must dry-run successfully")
	}
	if len(dryRun.Events) != 0 {
		t.Fatalf("a quiet contract must emit no events, got %+v", dryRun.Events)
	}
}

// TestCallContractNewVMFails deploys a slot whose source is NOT valid wasm
// (a commit stores the source verbatim; validation only happens at
// activate). callContract can still find the slot, but building the vm
// fails — the rpc surfaces that as an error
func TestCallContractNewVMFails(t *testing.T) {
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(key)

	mineViaRPC(t, node, key)

	// commit invalid wasm bytes: the commit path never compiles, so the
	// bogus source lands in the slot
	var commitHex string
	decodeInto(t, node.mustCall(t, "ng_genCommit", map[string]any{
		"fee": "0.05", "wasm": hex.EncodeToString([]byte{0x00, 0x61, 0x73, 0x6d, 0xde, 0xad}),
	}), &commitHex)
	signedHex := localSign(t, key, commitHex)
	node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": signedHex})
	mineViaRPC(t, node, key)

	// the slot exists (getContractInfo succeeds) but the vm cannot be built
	if _, rpcErr := node.call(t, "ng_callContract", map[string]any{"contract": addr.BS58()}); rpcErr == nil {
		t.Fatal("callContract on an uncompilable slot must fail at NewVM")
	}
}

// TestGetTxByHashBrokenRecord pokes a corrupt record into the tx bucket
// so GetTxByHash returns a decode error (NOT ErrKeyNotFound), driving
// getTxByHash's non-missing error arm
func TestGetTxByHashBrokenRecord(t *testing.T) {
	node := newRPCNode(t)

	badHash := make([]byte, 32)
	for i := range badHash {
		badHash[i] = 0x7a
	}

	err := node.pow.Chain.Update(func(txn *bbolt.Tx) error {
		return txn.Bucket(storage.TxBucketName).Put(badHash, []byte{0xff, 0xff})
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, rpcErr := node.call(t, "ng_getTxByHash",
		map[string]any{"hash": hex.EncodeToString(badHash)}); rpcErr == nil {
		t.Fatal("getTxByHash on a corrupt tx record must fail")
	}
}

// TestGetReceiptBrokenRecord corrupts the on-chain receipt of a mined tx,
// so getReceipt's GetTxRuns read fails and the rpc reports an error rather
// than an empty receipt
func TestGetReceiptBrokenRecord(t *testing.T) {
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	mineViaRPC(t, node, key)

	// a simple funded transact tx we can land on chain
	var unsignedHex string
	decodeInto(t, node.mustCall(t, "ng_genTransaction", map[string]any{
		"to": ngtypes.NewAddress(key).BS58(), "value": "1", "fee": "0.01",
	}), &unsignedHex)
	signedHex := localSign(t, key, unsignedHex)
	var txHash string
	decodeInto(t, node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": signedHex}), &txHash)
	mineViaRPC(t, node, key)

	rawHash, err := hex.DecodeString(txHash)
	if err != nil {
		t.Fatal(err)
	}

	// poison the receipt bucket for this tx: GetTxRuns must fail to decode
	err = node.pow.State.Update(func(txn *bbolt.Tx) error {
		return txn.Bucket(storage.ReceiptBucketName).Put(rawHash, []byte{0xff, 0xff})
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, rpcErr := node.call(t, "ng_getReceipt", map[string]any{"hash": txHash}); rpcErr == nil {
		t.Fatal("getReceipt on a corrupt receipt record must fail")
	}
}
