package rpc

import (
	"math/big"

	"github.com/maoxs2/go-jsonrpc2"

	"github.com/ngchain/ngcore/utils"
)

func (s *Server) getAccountsFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	key := utils.PublicKey2Bytes(*s.consensus.PrivateKey.PubKey())
	accounts, err := s.sheetManager.GetAccountsByPublicKey(key)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	result := make([]uint64, len(accounts))
	for i := range accounts {
		result[i] = accounts[i].Num
	}

	raw, err := utils.JSON.Marshal(result)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getBalanceReply struct {
	Balance *big.Int `json:"balance"`
}

func (s *Server) getBalanceFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	key := utils.PublicKey2Bytes(*s.consensus.PrivateKey.PubKey())
	balance, err := s.sheetManager.GetBalanceByPublicKey(key)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	raw, err := utils.JSON.Marshal(getBalanceReply{
		Balance: balance,
	})
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getBalanceByNumParams struct {
	Num uint64 `json:"id"`
}

func (s *Server) getBalanceByNumFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	params := new(getBalanceByNumParams)
	err := utils.JSON.Unmarshal(msg.Params, params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	balance, err := s.sheetManager.GetBalanceByNum(params.Num)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(getBalanceReply{
		Balance: balance,
	})
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getAccountByNumParams struct {
	Num uint64 `json:"id"`
}

func (s *Server) getAccountByNumFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	params := new(getAccountByNumParams)
	err := utils.JSON.Unmarshal(msg.Params, params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	account, err := s.sheetManager.GetAccountByNum(params.Num)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(account)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
