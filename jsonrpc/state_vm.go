package jsonrpc

import (
	jsonrpc2 "github.com/maoxs2/go-jsonrpc2"

	"github.com/ngchain/ngcore/utils"
	"github.com/ngchain/ngcore/vm"
)

// TODO: add options on machine joining, e.g. encryption
type runContractParams struct {
	Raw []byte `json:"raw"`
}

// runContractFunc typically used to run the long loop task. Can be treated as a deploy
func (s *Server) runContractFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params runContractParams
	err := utils.JSON.Unmarshal(msg.Params, params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	vm.NewWasmVM(params.Raw)

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}
