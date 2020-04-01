package rpc

import (
	"crypto/elliptic"

	"github.com/maoxs2/go-jsonrpc2"

	"github.com/ngchain/ngcore/utils"
)

func (s *Server) getAccountsFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	key := elliptic.Marshal(elliptic.P256(), s.consensus.PrivateKey.PublicKey.X, s.consensus.PrivateKey.PublicKey.Y)
	accounts := s.sheetManager.GetAccountsByPublicKey(key)
	raw, err := utils.Json.Marshal(accounts)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
