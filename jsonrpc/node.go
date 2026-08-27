package jsonrpc

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/c0mm4nd/go-jsonrpc2"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type nodeInfoReply struct {
	PeerID      string   `json:"peerId"`
	Protocol    string   `json:"protocol"`
	Network     string   `json:"network"`
	Version     string   `json:"version,omitempty"`
	Height      uint64   `json:"height"`
	Peers       int      `json:"peers"`
	ListenAddrs []string `json:"listenAddrs"`
}

// nodeInfoFunc reports the node's self-description for monitoring and
// explorers: identity, wired protocol, network, version and peer count
func (s *Server) nodeInfoFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	ln := s.pow.LocalNode

	addrs := ln.Addrs()
	listen := make([]string, len(addrs))
	for i, a := range addrs {
		listen[i] = a.String()
	}

	return reply(msg, nodeInfoReply{
		PeerID:      ln.ID().String(),
		Protocol:    string(ln.GetWiredProtocol()),
		Network:     s.pow.Network.String(),
		Version:     s.Version,
		Height:      s.pow.Chain.GetLatestBlockHeight(),
		Peers:       len(ln.Peerstore().PeersWithAddrs()),
		ListenAddrs: listen,
	})
}

// peerCountFunc is the count companion to admin_getPeers
func (s *Server) peerCountFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	return reply(msg, len(s.pow.LocalNode.Peerstore().PeersWithAddrs()))
}

type difficultyReply struct {
	Height uint64 `json:"height"`
	// Difficulty is the tip's DECLARED (consensus) difficulty — the network
	// target every block at this height must meet. ActualDifficulty is the
	// pow work the tip's winning nonce happened to achieve (>= Difficulty by
	// luck); it swings block to block and is NOT the network difficulty.
	Difficulty       string `json:"difficulty"`
	ActualDifficulty string `json:"actualDifficulty"`
	BlockReward      string `json:"blockReward"` // the tip height's reward, raw units
	NextReward       string `json:"nextReward"`  // the next height's reward, raw units
}

// getDifficultyFunc exposes the chain's current difficulty and block
// reward, for explorer front pages
func (s *Server) getDifficultyFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	tip, ok := s.pow.Chain.GetLatestBlock().(*ngtypes.FullBlock)
	if !ok {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, errors.New("latest block unavailable")))
	}
	height := tip.GetHeight()

	return reply(msg, difficultyReply{
		Height:           height,
		Difficulty:       new(big.Int).SetBytes(tip.BlockHeader.Difficulty).String(),
		ActualDifficulty: tip.GetActualDiff().String(),
		BlockReward:      ngtypes.GetBlockReward(height).String(),
		NextReward:       ngtypes.GetBlockReward(height + 1).String(),
	})
}

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
	// BaseFee is the consensus per-byte burn-only base fee the NEXT block will
	// carry (NextBaseFee from the chain tip), decimal raw units per wire byte.
	// Post-fork a tx must pay Fee >= BaseFee * len(rlp(tx)); the whole fee is
	// burned. Pre-fork this equals MinBaseFee.
	BaseFee string `json:"baseFee"`
	// MinFee, present when rawTx was given, is the least fee that tx must carry
	// to be both relayed AND accepted next block: max(minFeePerByte, baseFee) *
	// len(rawTx)
	MinFee string `json:"minFee,omitempty"`
}

// suggestFeeFunc exposes the relay fee floor AND the next-block consensus base
// fee so a wallet can price a tx before signing. Both scale with the tx's wire
// size; the least admissible fee is max(minFeePerByte, baseFee) * bytes.
func (s *Server) suggestFeeFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	perByte := s.pow.Pool.MinFeePerByte
	if perByte == nil {
		perByte = big.NewInt(0)
	}

	// the base fee the next block will carry, derived from the current tip, and
	// whether it is a HARD admission floor yet (only once the fork is active)
	baseFee := new(big.Int).Set(ngtypes.MinBaseFee)
	baseFeeEnforced := false
	if tip, ok := s.pow.Chain.GetLatestBlock().(*ngtypes.FullBlock); ok && tip != nil {
		baseFee = ngtypes.NextBaseFee(
			tip.BlockHeader.Network,
			tip.GetHeight(),
			new(big.Int).SetBytes(tip.BlockHeader.BaseFee),
			ngtypes.BlockUsedBytes(tip),
		)
		baseFeeEnforced = ngtypes.IsForkActive(
			tip.BlockHeader.Network, ngtypes.ForkFeeMarket, tip.GetHeight()+1)
	}

	// the effective per-byte floor a tx must clear to be relayed AND accepted:
	// max(relay policy, base fee) once the fork is active, else the relay policy
	floorPerByte := perByte
	if baseFeeEnforced && baseFee.Cmp(floorPerByte) > 0 {
		floorPerByte = baseFee
	}

	out := suggestFeeReply{
		MinFeePerByte: perByte.String(),
		BaseFee:       baseFee.String(),
	}

	// params are optional: no body just returns the per-byte rates
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
			out.MinFee = new(big.Int).Mul(floorPerByte, big.NewInt(int64(len(raw)))).String()
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
