package rpc

import (
	"github.com/maoxs2/go-jsonrpc2"
	"github.com/maoxs2/go-jsonrpc2/jsonrpc2http"
)

func NewHTTPHandler(s *Server) *jsonrpc2http.HTTPHandler {
	httpHandler := jsonrpc2http.NewHTTPHandler()

	httpHandler.RegisterJsonRpcHandleFunc("test", func(message *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
		return jsonrpc2.NewJsonRpcSuccess(message.ID, []byte("pong"))
	})

	httpHandler.RegisterJsonRpcHandleFunc("addnode", s.addNodeFunc)

	httpHandler.RegisterJsonRpcHandleFunc("sendregister", s.sendRegisterFunc)
	httpHandler.RegisterJsonRpcHandleFunc("sendlogout", s.sendLogoutFunc)
	httpHandler.RegisterJsonRpcHandleFunc("sendtransaction", s.sendTransactionFunc)
	httpHandler.RegisterJsonRpcHandleFunc("sendassign", s.sendAssignFunc)
	httpHandler.RegisterJsonRpcHandleFunc("sendappend", s.sendAppendFunc)

	httpHandler.RegisterJsonRpcHandleFunc("getaccounts", s.getAccountsFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getbalance", s.getBalanceFunc)

	return httpHandler
}
