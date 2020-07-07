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

// getBlockTemplateFunc provides the block template in JSON format for easier read and debug
func (s *Server) getBlockTemplateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	blockTemplate := consensus.GetPoWConsensus().GetBlockTemplate()

	raw, err := utils.JSON.Marshal(blockTemplate)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getWorkReply struct {
	PoWRaw string `json:"raw"`
}

// getBlockTemplateFunc provides the block template in JSON format for easier read and debug
func (s *Server) getWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	blockTemplate := consensus.GetPoWConsensus().GetBlockTemplate().GetPoWRawHeader(nil)

	raw, err := utils.JSON.Marshal(blockTemplate)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type submitWorkParams struct {
	RawHeader string `json:"header"`
	RawTxs    string `json:"txs"`
}

// getBlockTemplateFunc provides the block template in JSON format for easier read and debug
func (s *Server) submitWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params submitWorkParams

	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	// params.RawHeader

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}
