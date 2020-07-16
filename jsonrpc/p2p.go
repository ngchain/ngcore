package jsonrpc

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/maoxs2/go-jsonrpc2"
	"github.com/multiformats/go-multiaddr"

	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type addPeerParams struct {
	PeerMultiAddr string `json:"peerMultiAddr"`
}

func (s *Server) addPeerFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params addPeerParams

	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	targetAddr, err := multiaddr.NewMultiaddr(params.PeerMultiAddr)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	targetInfo, err := peer.AddrInfoFromP2pAddr(targetAddr)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = ngp2p.GetLocalNode().Connect(context.Background(), *targetInfo)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, nil)
}

func (s *Server) getNetworkFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	network, _ := utils.JSON.Marshal(ngtypes.NETWORK.String())
	return jsonrpc2.NewJsonRpcSuccess(msg.ID, network)
}

func (s *Server) getPeersFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	raw, _ := utils.JSON.Marshal(ngp2p.GetLocalNode().Peerstore().PeersWithAddrs())
	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
