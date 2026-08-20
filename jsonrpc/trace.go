package jsonrpc

import (
	"encoding/hex"

	"github.com/c0mm4nd/go-jsonrpc2"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/utils"
)

type traceTransactionParams struct {
	Hash string `json:"hash"`
}

// traceTransactionReply returns the tx's contract runs with their internal
// call/transfer trees (the "internal transactions"). Each run's trace is a
// pre-order flat list; depth reconstructs the tree. The trace is present
// even for a failed run, showing where it reverted.
type traceTransactionReply struct {
	OnChain     bool                  `json:"onChain"`
	BlockHeight uint64                `json:"blockHeight,omitempty"`
	Runs        []ngstate.ContractRun `json:"runs"`
}

func (s *Server) traceTransactionFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params traceTransactionParams
	if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	txHash, err := hex.DecodeString(params.Hash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	runs, err := s.pow.State.GetTxRuns(txHash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	if runs == nil {
		runs = []ngstate.ContractRun{}
	}

	out := traceTransactionReply{Runs: runs}
	if _, height, err := s.pow.Chain.GetTxLocation(txHash); err == nil {
		out.OnChain = true
		out.BlockHeight = height
	}

	return reply(msg, out)
}
