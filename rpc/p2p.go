package rpc

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/maoxs2/go-jsonrpc2"
	"github.com/multiformats/go-multiaddr"

	"github.com/ngchain/ngcore/utils"
)

func (s *Server) addNodeFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params string
	err := utils.Json.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	targetAddr, err := multiaddr.NewMultiaddr(params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	targetInfo, err := peer.AddrInfoFromP2pAddr(targetAddr)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.localNode.Connect(context.Background(), *targetInfo)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	s.localNode.Ping(targetInfo.ID)

	ok, _ := utils.Json.Marshal(true)
	return jsonrpc2.NewJsonRpcSuccess(msg.ID, ok)
}
