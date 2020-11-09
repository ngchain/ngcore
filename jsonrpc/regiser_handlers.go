package jsonrpc

import (
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
	s.RegisterJsonRpcHandleFunc("getLatestBlockHeight", s.getLatestBlockHeightFunc)
	s.RegisterJsonRpcHandleFunc("getLatestBlockHash", s.getLatestBlockHashFunc)
	s.RegisterJsonRpcHandleFunc("getLatestBlock", s.getLatestBlockFunc)
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

	s.RegisterJsonRpcHandleFunc("getAccountsByAddress", s.getAccountsByAddressFunc)
	s.RegisterJsonRpcHandleFunc("getAccountByNum", s.getAccountByNumFunc)
	s.RegisterJsonRpcHandleFunc("getBalanceByNum", s.getBalanceByNumFunc)
	s.RegisterJsonRpcHandleFunc("getBalanceByAddress", s.getBalanceByAddressFunc)

	// mining
	s.RegisterJsonRpcHandleFunc("submitBlock", s.submitBlockFunc)
	s.RegisterJsonRpcHandleFunc("getBlockTemplate", s.getBlockTemplateFunc)
	s.RegisterJsonRpcHandleFunc("getWork", s.getWorkFunc)
	s.RegisterJsonRpcHandleFunc("submitWork", s.submitWorkFunc)
	s.RegisterJsonRpcHandleFunc("switchMining", s.switchMiningFunc)

	// utils
	s.RegisterJsonRpcHandleFunc("publicKeyToAddress", s.publicKeyToAddressFunc)
}
