package rpc

import (
	"github.com/maoxs2/go-jsonrpc2"

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
		"balance": balance,
	})
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
