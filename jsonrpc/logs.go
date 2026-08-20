package jsonrpc

import (
	"encoding/hex"

	"github.com/c0mm4nd/go-jsonrpc2"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type getLogsParams struct {
	FromHeight uint64 `json:"fromHeight"`
	ToHeight   uint64 `json:"toHeight"` // 0 = up to the tip
	Address    string `json:"address"`  // bs58 emitter filter; "" = any
	Topic      string `json:"topic"`    // exact topic filter; "" = any
}

// logReply is one matched event with its on-chain location. A native
// internal transfer surfaces as topic "ng.transfer" with data = to(32) ‖
// value(32, LE); the emitter (contract) is the sender.
type logReply struct {
	Height   uint64 `json:"height"`
	TxHash   string `json:"txHash"`
	Contract string `json:"contract"`
	Topic    string `json:"topic"`
	Data     string `json:"data"`
	RunIndex int    `json:"runIndex"`
	LogIndex int    `json:"logIndex"`
}

// getLogsFunc scans the receipts in a block range and returns the events
// matching an optional emitter/topic — the external read for indexers and
// explorers. Internal transactions (contract value transfers) are included
// automatically as `ng.transfer` logs, no separate trace call needed.
func (s *Server) getLogsFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getLogsParams
	if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	filter := ngstate.LogFilter{FromHeight: params.FromHeight, ToHeight: params.ToHeight}
	if params.Address != "" {
		addr, err := ngtypes.NewAddressFromBS58(params.Address)
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		filter.Address = &addr
	}
	if params.Topic != "" {
		filter.Topic = &params.Topic
	}

	logs, err := s.pow.State.GetLogs(filter)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	out := make([]logReply, len(logs))
	for i, l := range logs {
		var addr ngtypes.Address
		copy(addr[:], l.Event.Contract)
		out[i] = logReply{
			Height:   l.Height,
			TxHash:   hex.EncodeToString(l.TxHash),
			Contract: addr.BS58(),
			Topic:    l.Event.Topic,
			Data:     hex.EncodeToString(l.Event.Data),
			RunIndex: l.RunIndex,
			LogIndex: l.LogIndex,
		}
	}

	return reply(msg, out)
}
