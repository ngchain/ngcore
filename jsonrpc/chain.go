package jsonrpc

import (
	"encoding/hex"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

func (s *Server) getLatestBlockHeightFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	height := s.pow.Chain.GetLatestBlockHeight()

	return reply(msg, height)
}

func (s *Server) getLatestBlockHashFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	hash := s.pow.Chain.GetLatestBlockHash()

	// human-facing raw bytes are lowercase hex, never base64
	return reply(msg, hex.EncodeToString(hash))
}

func (s *Server) getLatestBlockFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	block := s.pow.Chain.GetLatestBlock()

	return reply(msg, block)
}

type getBlockByHeightParams struct {
	Height uint64 `json:"height"`
}

func (s *Server) getBlockByHeightFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getBlockByHeightParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	block, err := s.pow.Chain.GetBlockByHeight(params.Height)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, block)
}

type getBlockByHashParams struct {
	Hash string `json:"hash"`
}

func (s *Server) getBlockByHashFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getBlockByHashParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	hash, err := hex.DecodeString(params.Hash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	block, err := s.pow.Chain.GetBlockByHash(hash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, block)
}

type getTxsByAddressParams struct {
	Address    string `json:"address"`
	FromHeight uint64 `json:"fromHeight"`
	ToHeight   uint64 `json:"toHeight"` // 0 = up to the tip
	Limit      int    `json:"limit"`    // 0 = a node-capped default
}

type getTxsByAddressReply struct {
	Count int               `json:"count"`
	Txs   []*ngtypes.FullTx `json:"txs"`
}

// getTxsByAddressFunc returns an address's transaction history (txs where
// it is the sender or the recipient), in height order — the account page
// for wallets and explorers
func (s *Server) getTxsByAddressFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getTxsByAddressParams
	if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	addr, err := ngtypes.NewAddressFromBS58(params.Address)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	txs, err := s.pow.Chain.GetTxsByAddress(addr, params.FromHeight, params.ToHeight, params.Limit)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, getTxsByAddressReply{Count: len(txs), Txs: txs})
}

type getTxByHashParams struct {
	Hash string `json:"hash"`
}

type getTxByHashReply struct {
	OnChain bool            `json:"onChain"`
	Tx      *ngtypes.FullTx `json:"tx"`

	// filled for on-chain txs via the tx->block index
	BlockHash     string `json:"blockHash,omitempty"`
	BlockHeight   uint64 `json:"blockHeight,omitempty"`
	Confirmations uint64 `json:"confirmations,omitempty"`
}

func (s *Server) getTxByHashFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getTxByHashParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	hash, err := hex.DecodeString(params.Hash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx, err := s.pow.Chain.GetTxByHash(hash)
	if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	if tx != nil {
		res := &getTxByHashReply{
			OnChain: true,
			Tx:      tx,
		}

		blockHash, height, err := s.pow.Chain.GetTxLocation(hash)
		if err == nil {
			res.BlockHash = hex.EncodeToString(blockHash)
			res.BlockHeight = height
			if latest := s.pow.Chain.GetLatestBlockHeight(); latest >= height {
				res.Confirmations = latest - height + 1
			}
		}

		return reply(msg, res)
	}

	// search in pool
	exists, tx := s.pow.Pool.IsInPool(hash)
	if exists && tx != nil {
		return reply(msg, &getTxByHashReply{
			OnChain: false,
			Tx:      tx,
		})
	}

	log.Error(err)
	return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
}
