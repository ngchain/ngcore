package jsonrpc

import (
	"encoding/hex"
	"strconv"
	"time"

	"github.com/c0mm4nd/go-jsonrpc2"

	"github.com/ngchain/ngcore/jsonrpc/workpool"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// type GetWorkParams struct {
// 	PrivateKey string `json:"private_key"`
// }

var workPool = workpool.GetWorkerPool()

type GetWorkReply struct {
	WorkID uint64 `json:"id"`
	Block  string `json:"block"`
	Txs    string `json:"txs"`
}

// getWorkFunc provides a free style interface for miner client getting latest block mining work
func (s *Server) getWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	block, txs := s.pow.GetBareBlockTemplateWithTxs()
	id := uint64(time.Now().UnixNano())
	reply := &GetWorkReply{
		WorkID: id,
		Block:  utils.HexRLPEncode(block),
		Txs:    utils.HexRLPEncode(txs),
	}

	workPool.Put(strconv.FormatUint(reply.WorkID, 10), reply)

	raw, err := utils.JSON.Marshal(reply)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type SubmitWorkParams struct {
	WorkID uint64 `json:"id"`
	Nonce  string `json:"nonce"`
	GenTx  string `json:"gen"`
}

// submitWorkFunc
func (s *Server) submitWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params SubmitWorkParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	originReply, err := workPool.Get(strconv.FormatUint(params.WorkID, 10))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	reply := originReply.(*GetWorkReply)

	nonce, err := hex.DecodeString(params.Nonce)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var genTx ngtypes.FullTx
	err = utils.HexRLPDecode(params.GenTx, &genTx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var block ngtypes.FullBlock
	var txs []*ngtypes.FullTx

	err = utils.HexRLPDecode(reply.Block, &block)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	err = utils.HexRLPDecode(reply.Txs, &txs)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = block.ToUnsealing(append([]*ngtypes.FullTx{&genTx}, txs...))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	err = block.ToSealed(nonce)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.pow.MinedNewBlock(&block)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}
