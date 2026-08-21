package jsonrpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/gorilla/websocket"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

// the rpc is access-controlled by deployment (bind host / firewall), not by
// browser origin, so any origin may open the socket
var wsUpgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

// serveWS upgrades to a WebSocket and serves jsonrpc over it: plain methods
// dispatch like HTTP, plus ng_subscribe / ng_unsubscribe and the pushed
// ng_subscription notifications. Every write goes through the single
// writeLoop goroutine (gorilla allows only one concurrent writer).
func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err)
		return
	}

	sess := &wsSession{conn: conn, send: make(chan []byte, 64), done: make(chan struct{}), server: s}
	go sess.writeLoop()
	sess.readLoop()
}

type wsSession struct {
	conn   *websocket.Conn
	send   chan []byte
	done   chan struct{}
	server *Server
}

func (sess *wsSession) writeLoop() {
	for {
		select {
		case raw := <-sess.send:
			if err := sess.conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				return
			}
		case <-sess.done:
			return
		}
	}
}

// push queues raw for the writer, dropping it if the client is too slow
// rather than blocking a chain/pool event. send is never closed, so this is
// safe even after the session has ended
func (sess *wsSession) push(raw []byte) {
	select {
	case sess.send <- raw:
	default:
	}
}

func (sess *wsSession) readLoop() {
	defer func() {
		sess.server.hub.removeSession(sess) // stop the event fan-out first
		close(sess.done)                    // then stop the writer
		_ = sess.conn.Close()
	}()

	for {
		_, raw, err := sess.conn.ReadMessage()
		if err != nil {
			return
		}
		if jsonrpc2.IsBatchMarshal(raw) {
			sess.dispatchBatch(raw)
			continue
		}
		msg, err := jsonrpc2.UnmarshalMessage(raw)
		if err != nil {
			continue
		}
		if resp := sess.server.dispatchWS(sess, msg); resp != nil {
			if b, err := json.Marshal(resp); err == nil {
				sess.push(b)
			}
		}
	}
}

// dispatchBatch handles a jsonrpc batch request over the WS transport
func (sess *wsSession) dispatchBatch(raw []byte) {
	batch, err := jsonrpc2.UnmarshalMessageBatch(raw)
	if err != nil {
		return
	}
	resBatch := make(jsonrpc2.JsonRpcMessageBatch, 0, len(batch))
	for _, m := range batch {
		if r := sess.server.dispatchWS(sess, m); r != nil {
			resBatch = append(resBatch, r)
		}
	}
	if len(resBatch) == 0 {
		return
	}
	if b, err := resBatch.Marshal(); err == nil {
		sess.push(b)
	}
}

// dispatchWS routes a WS request: the subscription verbs need the session,
// every other method reuses the shared registry
func (s *Server) dispatchWS(sess *wsSession, msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	switch msg.Method {
	case "ng_subscribe":
		return s.hub.subscribe(sess, msg)
	case "ng_unsubscribe":
		return s.hub.unsubscribe(sess, msg)
	default:
		if fn, ok := s.methods[msg.Method]; ok {
			return fn(msg)
		}
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, fmt.Errorf("method not found: %s", msg.Method)))
	}
}

// ---- subscription hub ----

type subKind string

const (
	subNewHeads   subKind = "newHeads"
	subLogs       subKind = "logs"
	subPendingTxs subKind = "pendingTxs"
)

type subscription struct {
	id      uint64
	sess    *wsSession
	kind    subKind
	address *ngtypes.Address // logs filter
	topic   *string          // logs filter
}

type subHub struct {
	server *Server

	mu     sync.Mutex
	nextID uint64
	subs   map[uint64]*subscription
}

func newSubHub(server *Server) *subHub {
	return &subHub{server: server, subs: make(map[uint64]*subscription)}
}

// install wires the block, reorg and mempool event sources to the hub
func (h *subHub) install() {
	h.server.pow.Chain.OnTipChanged = h.onTipChanged
	h.server.pow.Chain.OnReorg = h.onReorg
	h.server.pow.Pool.OnNewTx = h.onNewTx
}

type subscribeParams struct {
	Type    string `json:"type"`
	Address string `json:"address"` // logs only
	Topic   string `json:"topic"`   // logs only
}

func (h *subHub) subscribe(sess *wsSession, msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	if msg.Params == nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, errors.New("ng_subscribe needs a type")))
	}
	var p subscribeParams
	if err := json.Unmarshal(*msg.Params, &p); err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	sub := &subscription{sess: sess, kind: subKind(p.Type)}
	switch sub.kind {
	case subNewHeads, subPendingTxs:
	case subLogs:
		if p.Address != "" {
			addr, err := ngtypes.NewAddressFromBS58(p.Address)
			if err != nil {
				return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
			}
			sub.address = &addr
		}
		if p.Topic != "" {
			sub.topic = &p.Topic
		}
	default:
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, fmt.Errorf("unknown subscription type %q", p.Type)))
	}

	h.mu.Lock()
	h.nextID++
	sub.id = h.nextID
	h.subs[sub.id] = sub
	h.mu.Unlock()

	return reply(msg, subIDString(sub.id))
}

type unsubscribeParams struct {
	ID string `json:"id"`
}

func (h *subHub) unsubscribe(sess *wsSession, msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var p unsubscribeParams
	if msg.Params != nil {
		if err := json.Unmarshal(*msg.Params, &p); err != nil {
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
	}
	id, err := parseSubID(p.ID)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	h.mu.Lock()
	sub, ok := h.subs[id]
	if ok && sub.sess == sess {
		delete(h.subs, id)
	} else {
		ok = false
	}
	h.mu.Unlock()

	return reply(msg, ok)
}

func (h *subHub) removeSession(sess *wsSession) {
	h.mu.Lock()
	for id, sub := range h.subs {
		if sub.sess == sess {
			delete(h.subs, id)
		}
	}
	h.mu.Unlock()
}

// snapshot returns the subs of a kind so the push happens outside the lock
func (h *subHub) snapshot(kind subKind) []*subscription {
	h.mu.Lock()
	defer h.mu.Unlock()

	var out []*subscription
	for _, sub := range h.subs {
		if sub.kind == kind {
			out = append(out, sub)
		}
	}
	return out
}

type wsHead struct {
	Height    uint64 `json:"height"`
	Hash      string `json:"hash"`
	PrevHash  string `json:"prevHash"`
	Timestamp uint64 `json:"timestamp"`
}

// onTipChanged pushes the new head, and the matching logs, to subscribers
func (h *subHub) onTipChanged() {
	block := h.server.pow.Chain.GetLatestBlock()
	height := block.GetHeight()

	for _, sub := range h.snapshot(subNewHeads) {
		sub.sess.push(subNotification(sub.id, wsHead{
			Height:    height,
			Hash:      hex.EncodeToString(block.GetHash()),
			PrevHash:  hex.EncodeToString(block.GetPrevHash()),
			Timestamp: block.GetTimestamp(),
		}))
	}

	for _, sub := range h.snapshot(subLogs) {
		logs, err := h.server.pow.State.GetLogs(ngstate.LogFilter{
			FromHeight: height, ToHeight: height, Address: sub.address, Topic: sub.topic,
		})
		if err != nil {
			continue
		}
		for _, lg := range logs {
			sub.sess.push(subNotification(sub.id, logToReply(lg)))
		}
	}
}

// onReorg rolls a reorg out to matching logs subscribers: the orphaned
// blocks' logs marked removed (so an indexer rolls them back), then the
// logs the branch re-added below its new tip (the tip's own logs arrive
// separately via onTipChanged)
func (h *subHub) onReorg(removed, added []ngstate.Log) {
	subs := h.snapshot(subLogs)
	if len(subs) == 0 {
		return
	}
	emit := func(logs []ngstate.Log, wasRemoved bool) {
		for _, lg := range logs {
			for _, sub := range subs {
				if sub.address != nil && !bytes.Equal(lg.Event.Contract, sub.address[:]) {
					continue
				}
				if sub.topic != nil && lg.Event.Topic != *sub.topic {
					continue
				}
				r := logToReply(lg)
				r.Removed = wasRemoved
				sub.sess.push(subNotification(sub.id, r))
			}
		}
	}
	emit(removed, true)
	emit(added, false)
}

// onNewTx pushes the hash of a tx that just entered the mempool
func (h *subHub) onNewTx(tx *ngtypes.FullTx) {
	subs := h.snapshot(subPendingTxs)
	if len(subs) == 0 {
		return
	}
	hash := hex.EncodeToString(tx.GetHash())
	for _, sub := range subs {
		sub.sess.push(subNotification(sub.id, map[string]string{"hash": hash}))
	}
}

// subNotification builds an ng_subscription notification (a jsonrpc message
// with no id): {subscription, result}
func subNotification(id uint64, result interface{}) []byte {
	params, _ := json.Marshal(map[string]interface{}{
		"subscription": subIDString(id),
		"result":       result,
	})
	raw, _ := json.Marshal(jsonrpc2.NewJsonRpcNotification("ng_subscription", params))
	return raw
}

func subIDString(id uint64) string { return fmt.Sprintf("0x%x", id) }

func parseSubID(s string) (uint64, error) {
	if len(s) > 2 && s[:2] == "0x" {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}
