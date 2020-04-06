package rpc

import (
	"fmt"

	"github.com/maoxs2/go-jsonrpc2"
	"github.com/mr-tron/base58/base58"

	"github.com/ngchain/ngcore/utils"
)

func (s *Server) getAccountsFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	key := utils.ECDSAPublicKey2Bytes(s.consensus.PrivateKey.PublicKey)
	accounts, err := s.sheetManager.GetAccountsByPublicKey(key)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	result := make([]uint64, len(accounts))
	for i := range accounts {
		result[i] = accounts[i].ID
	}

	raw, err := utils.Json.Marshal(result)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

func (s *Server) getBalanceFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	key := utils.ECDSAPublicKey2Bytes(s.consensus.PrivateKey.PublicKey)
	balance, err := s.sheetManager.GetBalanceByPublicKey(key)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	raw, err := utils.Json.Marshal(map[string]interface{}{
		base58.FastBase58Encoding(key): balance,
	})
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getBalanceByIDParams struct {
	ID uint64 `json:"id"`
}

func (s *Server) getBalanceByIDFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	params := new(getBalanceByIDParams)
	err := utils.Json.Unmarshal(msg.Params, params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	balance, err := s.sheetManager.GetBalanceByID(params.ID)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.Json.Marshal(map[string]interface{}{
		fmt.Sprintf("%d", params.ID): balance,
	})
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
