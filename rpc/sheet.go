package rpc

import (
	"math/big"

	"github.com/maoxs2/go-jsonrpc2"

	"github.com/ngchain/ngcore/utils"
)

func (s *Server) getAccountsFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	key := utils.ECDSAPublicKey2Bytes(s.consensus.PrivateKey.PublicKey)
	accounts, err := s.sheetManager.GetAccountsByPublicKey(key)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	raw, err := utils.Json.Marshal(accounts)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

func (s *Server) getBalanceFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var balanceSheet = make(map[uint64]*big.Int)

	key := utils.ECDSAPublicKey2Bytes(s.consensus.PrivateKey.PublicKey)
	accounts, err := s.sheetManager.GetAccountsByPublicKey(key)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	for i := range accounts {
		balance, err := s.sheetManager.GetBalanceByID(accounts[i].ID)
		if err != nil {
			jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}

		balanceSheet[accounts[i].ID] = balance
	}
	raw, err := utils.Json.Marshal(balanceSheet)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
