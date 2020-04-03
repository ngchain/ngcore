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

	// httpHandler.RegisterJsonRpcHandleFunc("sendtoaddress", s.sendTxFunc)
	httpHandler.RegisterJsonRpcHandleFunc("sendtx", s.sendTxFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getaccounts", s.getAccountsFunc)
	httpHandler.RegisterJsonRpcHandleFunc("getbalance", s.getBalanceFunc)

	return httpHandler
}
