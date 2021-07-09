package jsonrpc

import (
	"encoding/hex"
	"math/big"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/ngchain/ngcore/jsonrpc/workpool"
	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type getBlockTemplateParams struct {
	PrivateKey string `json:"private_key"`
}

// getBlockTemplateFunc provides the whole block template in JSON format
// for further development e.g. customizing mining jobs.
func (s *Server) getBlockTemplateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getBlockTemplateParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	rawPrivateKey, err := hex.DecodeString(params.PrivateKey)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	privateKey := secp256k1.NewPrivateKey(new(big.Int).SetBytes(rawPrivateKey))
	blockTemplate := s.pow.GetBlockTemplate(privateKey)

	raw, err := utils.JSON.Marshal(blockTemplate)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

// submitBlockFunc receive the whole mined block and try to broadcast it.
func (s *Server) submitBlockFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var block ngtypes.Block

	err := utils.JSON.Unmarshal(*msg.Params, &block)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.pow.MinedNewBlock(&block)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, block.GetHash())
}

type getWorkParams struct {
	PrivateKey string `json:"private_key"`
}

var workPool = workpool.GetWorkerPool()

type getWorkReply struct {
	RawHeader string `json:"header"`
}

// getBlockTemplateFunc provides a more simple way to mining the block.
// The client will get a rlp-encoded block, just finding the location of bytes
func (s *Server) getWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getWorkParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	rawPrivateKey, err := hex.DecodeString(params.PrivateKey)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	privateKey := secp256k1.NewPrivateKey(new(big.Int).SetBytes(rawPrivateKey))
	blockTemplate := s.pow.GetBlockTemplate(privateKey)

	header := hex.EncodeToString(blockTemplate.GetPoWRawHeader(nil))

	workPool.Put(header, blockTemplate)

	reply := getWorkReply{
		RawHeader: header,
	}

	raw, err := utils.JSON.Marshal(reply)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type submitWorkParams struct {
	Nonce     string `json:"nonce"`
	RawHeader string `json:"header"`
}

// getBlockTemplateFunc provides the block template in JSON format for easier read and debug.
func (s *Server) submitWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params submitWorkParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	block, err := workPool.Get(params.RawHeader)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	nonce, err := hex.DecodeString(params.Nonce)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	block.Header.Nonce = nonce

	err = s.pow.MinedNewBlock(block)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}
