package jsonrpc

import (
	"encoding/hex"
	"fmt"

	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngpool"

	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngtypes"

	"github.com/maoxs2/go-jsonrpc2"
	"github.com/ngchain/ngcore/utils"
)

func (s *Server) getLatestBlockHeightFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	height := ngchain.GetLatestBlockHeight()

	raw, err := utils.JSON.Marshal(height)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

func (s *Server) getLatestBlockHashFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	hash := ngchain.GetLatestBlockHash()

	raw, err := utils.JSON.Marshal(hash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

func (s *Server) getLatestBlockFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	block := ngchain.GetLatestBlock()

	raw, err := utils.JSON.Marshal(block)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getBlockByHeightParams struct {
	Height uint64 `json:"height"`
}

func (s *Server) getBlockByHeightFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getBlockByHeightParams

	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	block, err := ngchain.GetBlockByHeight(params.Height)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(block)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getBlockByHashParams struct {
	Hash string `json:"hash"`
}

func (s *Server) getBlockByHashFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getBlockByHashParams

	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	hash, err := hex.DecodeString(params.Hash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	block, err := ngchain.GetBlockByHash(hash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(block)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getTxByHashParams struct {
	Hash string `json:"hash"`
}

type getTxByHashReply struct {
	OnChain bool        `json:"onChain"`
	Tx      *ngtypes.Tx `json:"tx"`
}

func (s *Server) getTxByHashFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getTxByHashParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	hash, err := hex.DecodeString(params.Hash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx, err := ngchain.GetTxByHash(hash)
	if err != nil && err != badger.ErrKeyNotFound {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	if tx != nil {
		raw, err := utils.JSON.Marshal(&getTxByHashReply{
			OnChain: true,
			Tx:      tx,
		})
		if err != nil {
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}

		return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
	}

	// search in pool
	exists, tx := ngpool.IsInPool(hash)
	if exists && tx != nil {
		raw, err := utils.JSON.Marshal(&getTxByHashReply{
			OnChain: false,
			Tx:      tx,
		})
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}

		return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
	}

	err = fmt.Errorf("cannot find the tx with hash %x", hash)
	log.Error(err)
	return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
}
