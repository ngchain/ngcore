package jsonrpc

// some utils for wallet clients

type getSchnorrPublicKeyParams struct {
	PrivateKeys []string
}

type getSchnorrPublicKeyReply struct {
	PublicKey string
}

// getSchnorrPublicKeyFunc helps client to get the schnorr publickey of private keys
// func (s *Server) getSchnorrPublicKeyFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
// 	var params getSchnorrPublicKeyParams
// 	err := utils.JSON.Unmarshal(msg.Params, &params)
// 	if err != nil {
// 		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
// 	}

// 	result := getSchnorrPublicKeyReply{
// 		PublicKey: publicKey,
// 	}
// 	raw, err := utils.JSON.Marshal(result)
// 	if err != nil {
// 		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
// 	}
// 	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
// }
