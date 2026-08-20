package jsonrpc

import (
	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/ngchain/ngcore/consensus"
)

// registerHTTPHandler will register jsonrpc functions onto the Server.
func registerHTTPHandler(s *Server) {
	s.RegisterJsonRpcHandleFunc("ping", func(message *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
		// the result must be valid json, or marshaling the response fails
		return jsonrpc2.NewJsonRpcSuccess(message.ID, []byte(`"pong"`))
	})

	// network & node admin (geth-style: net_ for chain info, admin_ for peers)
	if !s.DisableP2PMethods {
		s.RegisterJsonRpcHandleFunc("net_getNetwork", s.getNetworkFunc)
		s.RegisterJsonRpcHandleFunc("admin_addPeer", s.addPeerFunc)
		s.RegisterJsonRpcHandleFunc("admin_getPeers", s.getPeersFunc)
	}

	// chain
	s.RegisterJsonRpcHandleFunc("ng_getLatestBlockHeight", s.requireSynced(s.getLatestBlockHeightFunc))
	s.RegisterJsonRpcHandleFunc("ng_getLatestBlockHash", s.requireSynced(s.getLatestBlockHashFunc))
	s.RegisterJsonRpcHandleFunc("ng_getLatestBlock", s.requireSynced(s.getLatestBlockFunc))
	s.RegisterJsonRpcHandleFunc("ng_getBlockByHeight", s.getBlockByHeightFunc)
	s.RegisterJsonRpcHandleFunc("ng_getBlockByHash", s.getBlockByHashFunc)
	s.RegisterJsonRpcHandleFunc("ng_getTxByHash", s.getTxByHashFunc)

	// state
	s.RegisterJsonRpcHandleFunc("ng_sendTx", s.sendTxFunc)
	s.RegisterJsonRpcHandleFunc("ng_genDestroy", s.genDestroyFunc)
	s.RegisterJsonRpcHandleFunc("ng_genTransaction", s.genTransactionFunc)
	s.RegisterJsonRpcHandleFunc("ng_genCommit", s.genCommitFunc)
	s.RegisterJsonRpcHandleFunc("ng_genActivate", s.genActivateFunc)
	s.RegisterJsonRpcHandleFunc("ng_genDeactivate", s.genDeactivateFunc)
	s.RegisterJsonRpcHandleFunc("ng_callContract", s.callContractFunc)
	s.RegisterJsonRpcHandleFunc("ng_getReceipt", s.getReceiptFunc)
	// event/log query for indexers; internal transfers surface as ng.transfer
	s.RegisterJsonRpcHandleFunc("ng_getLogs", s.requireSynced(s.getLogsFunc))

	s.RegisterJsonRpcHandleFunc("ng_getContractInfo", s.requireSynced(s.getContractInfoFunc))
	// targeted contract kv read for external tools (indexers, wallets)
	s.RegisterJsonRpcHandleFunc("ng_getContractStorage", s.requireSynced(s.getContractStorageFunc))
	s.RegisterJsonRpcHandleFunc("ng_getBalanceByAddress", s.requireSynced(s.getBalanceByAddressFunc))
	// fork-chain sources: getHead + getAddressState back LAZY rpc forking,
	// getSheet the eager one-shot export
	s.RegisterJsonRpcHandleFunc("ng_getHead", s.requireSynced(s.getHeadFunc))
	s.RegisterJsonRpcHandleFunc("ng_getAddressState", s.requireSynced(s.getAddressStateFunc))
	s.RegisterJsonRpcHandleFunc("ng_getSheet", s.requireSynced(s.getSheetFunc))

	// mining
	if !s.DisableMiningMethods {
		// s.RegisterJsonRpcHandleFunc("ng_submitBlock", s.requireSynced(s.submitBlockFunc))
		// s.RegisterJsonRpcHandleFunc("ng_getBlockTemplate", s.requireSynced(s.getBlockTemplateFunc))
		s.RegisterJsonRpcHandleFunc("ng_getWork", s.requireSynced(s.getWorkFunc))       // dangerous: public key reveal
		s.RegisterJsonRpcHandleFunc("ng_submitWork", s.requireSynced(s.submitWorkFunc)) // dangerous: attack pow hash on verification
	}

	// utils
	s.RegisterJsonRpcHandleFunc("ng_publicKeyToAddress", s.publicKeyToAddressFunc)
}

func (s *Server) requireSynced(f func(*jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage) func(*jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	// the sync state must be sampled per request: at registration time it
	// would freeze whatever the node happened to be doing at startup
	return func(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
		if s.pow.SyncMod.IsActive() {
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, consensus.ErrChainOnSyncing))
		}

		return f(msg)
	}
}
