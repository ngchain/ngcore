package jsonrpc

import (
	"encoding/hex"
	"math/big"

	"github.com/c0mm4nd/go-jsonrpc2"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type syncingReply struct {
	Syncing bool   `json:"syncing"` // true while the sync module is catching up
	Height  uint64 `json:"height"`  // the node's current chain tip
}

// syncingFunc reports whether the node is still catching up. NOT gated by
// requireSynced — a client must be able to ask this while the node syncs
func (s *Server) syncingFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	return reply(msg, syncingReply{
		Syncing: s.pow.SyncMod.IsActive(),
		Height:  s.pow.Chain.GetLatestBlockHeight(),
	})
}

type suggestFeeParams struct {
	RawTx string `json:"rawTx"` // optional hex(rlp(tx)) to price the exact floor
}

type suggestFeeReply struct {
	// MinFeePerByte is this node's relay-policy floor, decimal raw units
	// per wire byte ("" when the floor is disabled)
	MinFeePerByte string `json:"minFeePerByte"`
	// MinFee, present when rawTx was given, is minFeePerByte * len(rawTx)
	// — the least fee that tx must carry to be relayed
	MinFee string `json:"minFee,omitempty"`
}

// suggestFeeFunc exposes the local relay fee floor so a wallet can price a
// tx before signing. The floor scales with the tx's wire size
func (s *Server) suggestFeeFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	perByte := s.pow.Pool.MinFeePerByte
	if perByte == nil {
		perByte = big.NewInt(0)
	}
	out := suggestFeeReply{MinFeePerByte: perByte.String()}

	// params are optional: no body just returns the per-byte rate
	if msg.Params != nil {
		var params suggestFeeParams
		if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		if params.RawTx != "" {
			raw, err := hex.DecodeString(params.RawTx)
			if err != nil {
				log.Error(err)
				return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
			}
			out.MinFee = new(big.Int).Mul(perByte, big.NewInt(int64(len(raw)))).String()
		}
	}

	return reply(msg, out)
}

type pendingTxsReply struct {
	Count int               `json:"count"`
	Txs   []*ngtypes.FullTx `json:"txs"`
}

// getPendingTxsFunc lists the txs queued in this node's mempool (at most
// one per From address), for explorers and wallets tracking submissions
func (s *Server) getPendingTxsFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	txs := s.pow.Pool.List()
	return reply(msg, pendingTxsReply{Count: len(txs), Txs: txs})
}
