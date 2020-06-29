package jsonrpc

import (
	"github.com/maoxs2/go-jsonrpc2"
	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (s *Server) submitBlockFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var block ngtypes.Block
	err := utils.JSON.Unmarshal(msg.Params, &block)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = consensus.GetPoWConsensus().MinedNewBlock(&block)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, block.Hash())
}

func (s *Server) getBlockTemplateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	blockTemplate := consensus.GetPoWConsensus().GetBlockTemplate()

	raw, err := utils.JSON.Marshal(blockTemplate)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
