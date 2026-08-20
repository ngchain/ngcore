package jsonrpc

import (
	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/ngchain/ngcore/consensus"
)

// registerHTTPHandler will register jsonrpc functions onto the Server.
func registerHTTPHandler(s *Server) {
	s.reg("ping", func(message *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
		// the result must be valid json, or marshaling the response fails
		return jsonrpc2.NewJsonRpcSuccess(message.ID, []byte(`"pong"`))
	})

	// network & node admin (geth-style: net_ for chain info, admin_ for peers)
	if !s.DisableP2PMethods {
		s.reg("net_getNetwork", s.getNetworkFunc)
		s.reg("admin_addPeer", s.addPeerFunc)
		s.reg("admin_getPeers", s.getPeersFunc)
	}

	// chain
	s.reg("ng_getLatestBlockHeight", s.requireSynced(s.getLatestBlockHeightFunc))
	s.reg("ng_getLatestBlockHash", s.requireSynced(s.getLatestBlockHashFunc))
	s.reg("ng_getLatestBlock", s.requireSynced(s.getLatestBlockFunc))
	s.reg("ng_getBlockByHeight", s.getBlockByHeightFunc)
	s.reg("ng_getBlockByHash", s.getBlockByHashFunc)
	s.reg("ng_getTxByHash", s.getTxByHashFunc)

	// state
	s.reg("ng_sendTx", s.sendTxFunc)
	s.reg("ng_genDestroy", s.genDestroyFunc)
	s.reg("ng_genTransaction", s.genTransactionFunc)
	s.reg("ng_genCommit", s.genCommitFunc)
	s.reg("ng_genActivate", s.genActivateFunc)
	s.reg("ng_genDeactivate", s.genDeactivateFunc)
	s.reg("ng_callContract", s.callContractFunc)
	s.reg("ng_getReceipt", s.getReceiptFunc)
	// event/log query for indexers; internal transfers surface as ng.transfer
	s.reg("ng_getLogs", s.requireSynced(s.getLogsFunc))
	// per-tx internal call/transfer trace (the internal-transactions tree)
	s.reg("ng_traceTransaction", s.requireSynced(s.traceTransactionFunc))
	s.reg("ng_traceBlock", s.requireSynced(s.traceBlockFunc))

	s.reg("ng_getContractInfo", s.requireSynced(s.getContractInfoFunc))
	// targeted contract kv read for external tools (indexers, wallets)
	s.reg("ng_getContractStorage", s.requireSynced(s.getContractStorageFunc))
	s.reg("ng_getBalanceByAddress", s.requireSynced(s.getBalanceByAddressFunc))
	// fork-chain sources: getHead + getAddressState back LAZY rpc forking,
	// getSheet the eager one-shot export
	s.reg("ng_getHead", s.requireSynced(s.getHeadFunc))
	s.reg("ng_getAddressState", s.requireSynced(s.getAddressStateFunc))
	s.reg("ng_getSheet", s.requireSynced(s.getSheetFunc))

	// mining
	if !s.DisableMiningMethods {
		// s.reg("ng_submitBlock", s.requireSynced(s.submitBlockFunc))
		// s.reg("ng_getBlockTemplate", s.requireSynced(s.getBlockTemplateFunc))
		s.reg("ng_getWork", s.requireSynced(s.getWorkFunc))       // dangerous: public key reveal
		s.reg("ng_submitWork", s.requireSynced(s.submitWorkFunc)) // dangerous: attack pow hash on verification
	}

	// node status & mempool (ungated: a client may query these while syncing)
	s.reg("ng_syncing", s.syncingFunc)
	s.reg("ng_suggestFee", s.suggestFeeFunc)
	s.reg("ng_getPendingTxs", s.getPendingTxsFunc)

	// utils
	s.reg("ng_publicKeyToAddress", s.publicKeyToAddressFunc)
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
