package jsonrpc

import (
	"github.com/maoxs2/go-jsonrpc2"
	"github.com/maoxs2/go-jsonrpc2/jsonrpc2http"
)

// newHTTPHandler will create a jsonrpc2http.HTTPHandler struct and register jsonrpc functions onto it.
func newHTTPHandler(s *Server) *jsonrpc2http.HTTPHandler {
	httpHandler := jsonrpc2http.NewHTTPHandler()

	httpHandler.RegisterJsonRpcHandleFunc("ping", func(message *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
		return jsonrpc2.NewJsonRpcSuccess(message.ID, []byte("pong"))
	})

	// p2p
	httpHandler.RegisterJsonRpcHandleFunc("addNode", s.addPeerFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getPeers", s.getPeersFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getNetwork", s.getNetworkFunc)

	// chain
	httpHandler.RegisterJsonRpcHandleFunc("getLatestBlockHeight", s.getLatestBlockHeightFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getLatestBlockHash", s.getLatestBlockHeightFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getLatestBlock", s.getLatestBlockFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getBlockByHeight", s.getBlockByHeightFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getBlockByHash", s.getBlockByHashFunc)

	// state
	httpHandler.RegisterJsonRpcHandleFunc("sendTx", s.sendTxFunc)
	httpHandler.RegisterJsonRpcHandleFunc("signTx", s.signTxFunc)
	httpHandler.RegisterJsonRpcHandleFunc("genRegister", s.genRegisterFunc)
	httpHandler.RegisterJsonRpcHandleFunc("genLogout", s.genLogoutFunc)
	httpHandler.RegisterJsonRpcHandleFunc("genTransaction", s.genTransactionFunc)
	httpHandler.RegisterJsonRpcHandleFunc("genAssign", s.genAssignFunc)
	httpHandler.RegisterJsonRpcHandleFunc("genAppend", s.genAppendFunc)

	httpHandler.RegisterJsonRpcHandleFunc("getAccountsByAddress", s.getAccountsByAddressFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getBalanceByNum", s.getBalanceByNumFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getBalanceByAddress", s.getBalanceByAddressFunc)

	// mining
	httpHandler.RegisterJsonRpcHandleFunc("submitBlock", s.submitBlockFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getBlockTemplate", s.getBlockTemplateFunc)

	return httpHandler
}
