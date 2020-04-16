package rpc

import (
	"github.com/maoxs2/go-jsonrpc2"

	"github.com/ngchain/ngcore/utils"
	"github.com/ngchain/ngcore/vm"
)

// TODO: add options on machine joining, e.g. encryption
type runContractParams struct {
	Num    uint64        `json:"num"`
	Params []interface{} `json:"params"`
}

// runContractFunc typically used to run the long loop task. Can be treated as a deploy
func (s *Server) runContractFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params runContractParams
	err := utils.JSON.Unmarshal(msg.Params, params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	account, err := s.sheetManager.GetAccountByNum(params.Num)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	newVM, err := vm.NewWasmVM(account.Contract)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	newVM.RunDeploy(params.Params...)

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}
