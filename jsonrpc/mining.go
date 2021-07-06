package jsonrpc

import (
	"encoding/hex"
	"math/big"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

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

type getBlockTemplateParams struct {
	PrivateKey string `json:"private_key"`
}

// getBlockTemplateFunc provides the block template in JSON format for easier read and debug.
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

type getWorkParams struct {
	PrivateKey string `json:"private_key"`
}

type getWorkReply struct {
	RawHeader string `json:"raw"` // seed is in [17:49]
	RawBlock  string `json:"block"`
}

// getBlockTemplateFunc provides the block template in JSON format for easier read and debug.
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

	rawBlock, err := rlp.EncodeToBytes(blockTemplate)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	reply := getWorkReply{
		RawHeader: hex.EncodeToString(blockTemplate.GetPoWRawHeader(nil)),
		RawBlock:  hex.EncodeToString(rawBlock),
	}

	raw, err := utils.JSON.Marshal(reply)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type submitWorkParams struct {
	Nonce    string `json:"nonce"`
	RawBlock string `json:"block"`
}

// getBlockTemplateFunc provides the block template in JSON format for easier read and debug.
func (s *Server) submitWorkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params submitWorkParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	nonce, err := hex.DecodeString(params.Nonce)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	rawBlock, err := hex.DecodeString(params.RawBlock)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var newBlock ngtypes.Block
	err = rlp.DecodeBytes(rawBlock, &newBlock)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	newBlock.Header.Nonce = nonce

	err = s.pow.MinedNewBlock(&newBlock)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}
