package rpc

import (
	"github.com/maoxs2/go-jsonrpc2"
	"github.com/maoxs2/go-jsonrpc2/jsonrpc2http"
)

// newHTTPHandler will create a jsonrpc2http.HTTPHandler struct and register jsonrpc functions onto it.
func newHTTPHandler(s *Server) *jsonrpc2http.HTTPHandler {
	httpHandler := jsonrpc2http.NewHTTPHandler()

	httpHandler.RegisterJsonRpcHandleFunc("test", func(message *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
		return jsonrpc2.NewJsonRpcSuccess(message.ID, []byte("pong"))
	})

	httpHandler.RegisterJsonRpcHandleFunc("addNode", s.addNodeFunc)

	httpHandler.RegisterJsonRpcHandleFunc("sendRegister", s.sendRegisterFunc)
	httpHandler.RegisterJsonRpcHandleFunc("sendLogout", s.sendLogoutFunc)
	httpHandler.RegisterJsonRpcHandleFunc("sendTransaction", s.sendTransactionFunc)
	httpHandler.RegisterJsonRpcHandleFunc("sendAssign", s.sendAssignFunc)
	httpHandler.RegisterJsonRpcHandleFunc("sendAppend", s.sendAppendFunc)

	httpHandler.RegisterJsonRpcHandleFunc("getAccounts", s.getAccountsFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getBalanceByNum", s.getBalanceByNumFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getBalance", s.getBalanceFunc)

	return httpHandler
}
