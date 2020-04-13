package rpc

import (
	"github.com/maoxs2/go-jsonrpc2"

	"github.com/ngchain/ngcore/utils"
	"github.com/ngchain/ngcore/vm"
)

// TODO: add options on machine joining, e.g. encryption
type runStateParams struct {
	Num uint64 `json:"num"`
}

func (s *Server) runStateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params runStateParams
	err := utils.JSON.Unmarshal(msg.Params, params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	account, err := s.sheetManager.GetAccountByNum(params.Num)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	js := vm.NewJSVM()
	go js.RunState(account.State)

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}
