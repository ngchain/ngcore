package rpc

import (
	"github.com/maoxs2/go-jsonrpc2"
	"github.com/maoxs2/go-jsonrpc2/jsonrpc2http"
	"github.com/ngchain/ngcore/utils"
)

func NewHTTPHandler(s *Server) *jsonrpc2http.HTTPHandler {
	httpHandler := jsonrpc2http.NewHTTPHandler()

	httpHandler.RegisterJsonRpcHandleFunc("test", func(message *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
		result, _ := utils.Json.Marshal("pong")
		return jsonrpc2.NewJsonRpcSuccess(message.ID, result)
	})

	httpHandler.RegisterJsonRpcHandleFunc("addnode", s.AddNode)

	return httpHandler
}
