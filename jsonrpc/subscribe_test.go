package jsonrpc_test

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/ngchain/ngcore/ngtypes"
)

func dialWS(t *testing.T, node *rpcNode) *websocket.Conn {
	t.Helper()
	url := "ws://" + strings.TrimPrefix(node.url, "http://") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// wsCall sends a request and returns the matching response, skipping any
// notifications that arrive in between
func wsCall(t *testing.T, conn *websocket.Conn, id int, method string, params any) map[string]json.RawMessage {
	t.Helper()
	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		var m map[string]json.RawMessage
		if err := conn.ReadJSON(&m); err != nil {
			t.Fatalf("ws read: %v", err)
		}
		if _, isResp := m["id"]; isResp {
			return m
		}
	}
}

// TestRPCWebSocket drives the WS transport: a regular method works, and a
// newHeads subscription receives a push when a block is mined over HTTP
func TestRPCWebSocket(t *testing.T) {
	node := newRPCNode(t)
	conn := dialWS(t, node)

	// a plain method is callable over the WS connection too
	resp := wsCall(t, conn, 1, "ng_syncing", nil)
	if !strings.Contains(string(resp["result"]), "syncing") {
		t.Fatalf("ng_syncing over ws = %s", resp["result"])
	}

	// subscribe to new heads
	resp = wsCall(t, conn, 2, "ng_subscribe", map[string]any{"type": "newHeads"})
	var subID string
	if err := json.Unmarshal(resp["result"], &subID); err != nil || subID == "" {
		t.Fatalf("subscribe result = %s (%v)", resp["result"], err)
	}

	// mining a block over HTTP must push a newHeads notification over WS
	miner, _ := ngtypes.GenerateKey()
	mineViaRPC(t, node, miner)

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var notif struct {
		Method string `json:"method"`
		Params struct {
			Subscription string `json:"subscription"`
			Result       struct {
				Height uint64 `json:"height"`
			} `json:"result"`
		} `json:"params"`
	}
	for {
		if err := conn.ReadJSON(&notif); err != nil {
			t.Fatalf("ws notification read: %v", err)
		}
		if notif.Method == "ng_subscription" {
			break
		}
	}
	if notif.Params.Subscription != subID {
		t.Fatalf("notification subscription = %s, want %s", notif.Params.Subscription, subID)
	}
	if notif.Params.Result.Height != 1 {
		t.Fatalf("newHeads height = %d, want 1", notif.Params.Result.Height)
	}

	// unsubscribe succeeds
	resp = wsCall(t, conn, 3, "ng_unsubscribe", map[string]any{"id": subID})
	if string(resp["result"]) != "true" {
		t.Fatalf("unsubscribe = %s, want true", resp["result"])
	}
}

// TestRPCWebSocketLogs drives the logs subscription with an address+topic
// filter: a contract that emits must push a matching log over WS
func TestRPCWebSocketLogs(t *testing.T) {
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
	mineViaRPC(t, node, key)
	send("ng_genCommit", map[string]any{"fee": "0.05", "wasm": hex.EncodeToString(mustWat(contractWat))})
	mineViaRPC(t, node, key)
	send("ng_genActivate", map[string]any{"fee": "0.05"})
	mineViaRPC(t, node, key)
	send("ng_genTransaction", map[string]any{"to": addr.BS58(), "value": "0", "fee": "0.01"})
	mineViaRPC(t, node, key) // runs main -> emits the "key" log

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var notif struct {
		Method string `json:"method"`
		Params struct {
			Result struct {
				Contract string `json:"contract"`
				Topic    string `json:"topic"`
				Data     string `json:"data"`
			} `json:"result"`
		} `json:"params"`
	}
	for {
		if err := conn.ReadJSON(&notif); err != nil {
			t.Fatalf("ws logs read: %v", err)
		}
		if notif.Method == "ng_subscription" {
			break
		}
	}
	if notif.Params.Result.Topic != "key" || notif.Params.Result.Contract != addr.BS58() {
		t.Fatalf("log notification = %+v", notif.Params.Result)
	}
}

// TestRPCWebSocketErrors covers the subscribe/unsubscribe/dispatch error
// paths over WS
func TestRPCWebSocketErrors(t *testing.T) {
	node := newRPCNode(t)
	conn := dialWS(t, node)

	hasError := func(resp map[string]json.RawMessage) bool { _, ok := resp["error"]; return ok }

	if !hasError(wsCall(t, conn, 1, "ng_nope", nil)) {
		t.Error("unknown method must error")
	}
	if !hasError(wsCall(t, conn, 2, "ng_subscribe", nil)) {
		t.Error("subscribe without params must error")
	}
	if !hasError(wsCall(t, conn, 3, "ng_subscribe", map[string]any{"type": "bogus"})) {
		t.Error("unknown subscription type must error")
	}
	if !hasError(wsCall(t, conn, 4, "ng_subscribe", map[string]any{"type": "logs", "address": "0OIl"})) {
		t.Error("logs subscribe with a bad address must error")
	}
	if !hasError(wsCall(t, conn, 5, "ng_unsubscribe", map[string]any{"id": "zz"})) {
		t.Error("unsubscribe with a non-numeric id must error")
	}
	// a decimal, unknown id: parses but returns false (exercises the decimal branch)
	if resp := wsCall(t, conn, 6, "ng_unsubscribe", map[string]any{"id": "999"}); string(resp["result"]) != "false" {
		t.Fatalf("unsubscribe unknown id = %s, want false", resp["result"])
	}
}

// TestRPCWebSocketPendingTxs drives the pendingTxs subscription: a tx
// broadcast over HTTP must push a hash notification over WS
func TestRPCWebSocketPendingTxs(t *testing.T) {
	node := newRPCNode(t)
	key, _ := ngtypes.GenerateKey()
	mineViaRPC(t, node, key) // fund

	conn := dialWS(t, node)
	resp := wsCall(t, conn, 1, "ng_subscribe", map[string]any{"type": "pendingTxs"})
	var subID string
	_ = json.Unmarshal(resp["result"], &subID)

	// compose + sign + broadcast a tx over HTTP
	var unsigned string
	decodeInto(t, node.mustCall(t, "ng_genTransaction", map[string]any{
		"to": ngtypes.NewAddress(key).BS58(), "value": "1", "fee": "0.01",
	}), &unsigned)
	node.mustCall(t, "ng_sendTx", map[string]any{"rawTx": localSign(t, key, unsigned)})

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var notif struct {
		Method string `json:"method"`
		Params struct {
			Result struct {
				Hash string `json:"hash"`
			} `json:"result"`
		} `json:"params"`
	}
	for {
		if err := conn.ReadJSON(&notif); err != nil {
			t.Fatalf("ws pendingTx read: %v", err)
		}
		if notif.Method == "ng_subscription" {
			break
		}
	}
	if notif.Params.Result.Hash == "" {
		t.Fatal("pendingTxs notification carried no hash")
	}
}
