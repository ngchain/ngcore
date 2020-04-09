package rpc

import (
	"github.com/maoxs2/go-jsonrpc2"

	"github.com/ngchain/ngcore/utils"
)

func (s *Server) runState(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	key := utils.PublicKey2Bytes(*s.consensus.PrivateKey.PubKey())
	accounts, err := s.sheetManager.GetAccountsByPublicKey(key)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	result := make([]uint64, len(accounts))
	for i := range accounts {
		result[i] = accounts[i].ID
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}
