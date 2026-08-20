package jsonrpc_test

import (
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
