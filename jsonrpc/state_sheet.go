package jsonrpc

import (
	"github.com/mr-tron/base58"
	"github.com/ngchain/ngcore/ngtypes"
	"math/big"

	"github.com/maoxs2/go-jsonrpc2"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/utils"
)

func (s *Server) getAccountsFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	addr, err := base58.FastBase58Decoding(string(msg.Params))
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	accounts, err := ngstate.GetCurrentState().GetAccountsByAddress(addr)
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
	var addr ngtypes.Address
	if len(msg.Params) == 35 {
		bAddr, err := base58.FastBase58Decoding(string(msg.Params))
		if err != nil {
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		addr = ngtypes.Address(bAddr)
	} else {
		var num uint64
		err := utils.JSON.Unmarshal(msg.Params, &num)
		if err != nil {
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		acc, err := ngstate.GetCurrentState().GetAccountByNum(num)
		if err != nil {
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		addr = ngtypes.Address(acc.Owner)
	}

	balance, err := ngstate.GetCurrentState().GetBalanceByAddress(addr)
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

func (s *Server) getBalanceByNumFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var num uint64
	err := utils.JSON.Unmarshal(msg.Params, &num)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	balance, err := ngstate.GetCurrentState().GetBalanceByNum(num)
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

func (s *Server) getAccountByNumFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var num uint64
	err := utils.JSON.Unmarshal(msg.Params, &num)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	account, err := ngstate.GetCurrentState().GetAccountByNum(num)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(account)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
