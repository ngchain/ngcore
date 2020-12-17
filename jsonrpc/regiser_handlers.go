package jsonrpc

import (
	"fmt"

	"github.com/c0mm4nd/go-jsonrpc2"
)

// registerHTTPHandler will register jsonrpc functions onto the Server.
func registerHTTPHandler(s *Server) {
	s.RegisterJsonRpcHandleFunc("ping", func(message *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
		return jsonrpc2.NewJsonRpcSuccess(message.ID, []byte("pong"))
	})

	// p2p
	s.RegisterJsonRpcHandleFunc("addNode", s.addPeerFunc) // keep this alia
	s.RegisterJsonRpcHandleFunc("addPeer", s.addPeerFunc)
	s.RegisterJsonRpcHandleFunc("getNodes", s.getPeersFunc) // keep this alia
	s.RegisterJsonRpcHandleFunc("getPeers", s.getPeersFunc)
	s.RegisterJsonRpcHandleFunc("getNetwork", s.getNetworkFunc)

	// chain
	s.RegisterJsonRpcHandleFunc("getLatestBlockHeight", s.requireSynced(s.getLatestBlockHeightFunc))
	s.RegisterJsonRpcHandleFunc("getLatestBlockHash", s.requireSynced(s.getLatestBlockHashFunc))
	s.RegisterJsonRpcHandleFunc("getLatestBlock", s.requireSynced(s.getLatestBlockFunc))
	s.RegisterJsonRpcHandleFunc("getBlockByHeight", s.getBlockByHeightFunc)
	s.RegisterJsonRpcHandleFunc("getBlockByHash", s.getBlockByHashFunc)

	s.RegisterJsonRpcHandleFunc("getTxByHash", s.getTxByHashFunc)

	// state
	s.RegisterJsonRpcHandleFunc("sendTx", s.sendTxFunc)
	s.RegisterJsonRpcHandleFunc("signTx", s.signTxFunc)
	s.RegisterJsonRpcHandleFunc("genRegister", s.genRegisterFunc)
	s.RegisterJsonRpcHandleFunc("genLogout", s.genLogoutFunc)
	s.RegisterJsonRpcHandleFunc("genTransaction", s.genTransactionFunc)
	s.RegisterJsonRpcHandleFunc("genAssign", s.genAssignFunc)
	s.RegisterJsonRpcHandleFunc("genAppend", s.genAppendFunc)

	s.RegisterJsonRpcHandleFunc("getAccountByAddress", s.requireSynced(s.getAccountByAddressFunc))
	s.RegisterJsonRpcHandleFunc("getAccountByNum", s.requireSynced(s.getAccountByNumFunc))
	s.RegisterJsonRpcHandleFunc("getBalanceByNum", s.requireSynced(s.getBalanceByNumFunc))
	s.RegisterJsonRpcHandleFunc("getBalanceByAddress", s.requireSynced(s.getBalanceByAddressFunc))

	// mining
	s.RegisterJsonRpcHandleFunc("submitBlock", s.requireSynced(s.submitBlockFunc))
	s.RegisterJsonRpcHandleFunc("getBlockTemplate", s.requireSynced(s.getBlockTemplateFunc))
	s.RegisterJsonRpcHandleFunc("getWork", s.requireSynced(s.getWorkFunc))
	s.RegisterJsonRpcHandleFunc("submitWork", s.requireSynced(s.submitWorkFunc))
	s.RegisterJsonRpcHandleFunc("switchMining", s.requireSynced(s.switchMiningFunc))

	// utils
	s.RegisterJsonRpcHandleFunc("publicKeyToAddress", s.publicKeyToAddressFunc)
}

func (s *Server) requireSynced(f func(*jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage) func(*jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	if s.pow.SyncMod.IsLocked() {
		return func(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, fmt.Errorf("chain is syncing")))
		}
	}

	return f
}
