package jsonrpc

import (
	"encoding/hex"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
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

type traceBlockParams struct {
	Height uint64 `json:"height"`
}

type txTrace struct {
	TxHash string                `json:"txHash"`
	Runs   []ngstate.ContractRun `json:"runs"`
}

type traceBlockReply struct {
	Height    uint64    `json:"height"`
	BlockHash string    `json:"blockHash"`
	Txs       []txTrace `json:"txs"`
}

// traceBlockFunc returns the internal call/transfer traces of every tx in
// a block that ran a contract (txs with no runs are omitted).
func (s *Server) traceBlockFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params traceBlockParams
	if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	block, err := s.pow.Chain.GetBlockByHeight(params.Height)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	full, ok := block.(*ngtypes.FullBlock)
	if !ok {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, errors.New("block carries no txs")))
	}

	out := traceBlockReply{Height: params.Height, BlockHash: hex.EncodeToString(block.GetHash()), Txs: []txTrace{}}
	for _, tx := range full.Txs {
		txHash := tx.GetHash()
		runs, err := s.pow.State.GetTxRuns(txHash)
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		if len(runs) == 0 {
			continue
		}
		out.Txs = append(out.Txs, txTrace{TxHash: hex.EncodeToString(txHash), Runs: runs})
	}

	return reply(msg, out)
}
