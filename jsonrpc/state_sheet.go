package jsonrpc

import (
	"encoding/hex"
	"math/big"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// getSheetReply carries a full-state export: the whole sheet (balances,
// contracts with code+context, key registry) as hex RLP, plus the head
// it was taken at. `ngcore fork --rpc` rebuilds an identical local
// chain from this ONE response — rpc-based forking, anvil style.
type getSheetReply struct {
	Network   uint8  `json:"network"`
	Height    uint64 `json:"height"`
	BlockHash string `json:"blockHash"`
	Timestamp uint64 `json:"timestamp"` // the head block's time
	Sheet     string `json:"sheet"`     // hex(rlp(ngtypes.Sheet))
}

// getHeadReply is the light fork-boot info: what `ngcore fork --rpc`
// (lazy mode) needs before fetching any address
type getHeadReply struct {
	Network   uint8  `json:"network"`
	Height    uint64 `json:"height"`
	BlockHash string `json:"blockHash"`
	Timestamp uint64 `json:"timestamp"`
}

func (s *Server) getHeadFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	latest := s.pow.Chain.GetLatestBlock()

	return reply(msg, getHeadReply{
		Network:   uint8(s.pow.State.Network),
		Height:    latest.GetHeight(),
		BlockHash: hex.EncodeToString(latest.GetHash()),
		Timestamp: latest.GetTimestamp(),
	})
}

// getAddressStateReply is one address's full state — the unit of LAZY
// rpc forking: `ngcore fork --rpc` pulls addresses on first touch instead
// of the whole sheet, so forking scales with what the debug session
// actually reads, not with chain size
type getAddressStateReply struct {
	Exists   bool   `json:"exists"`
	Balance  string `json:"balance"`            // decimal raw units
	Contract string `json:"contract,omitempty"` // hex(rlp(ngtypes.Contract)); "" when no slot
}

func (s *Server) getAddressStateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params addressParams
	if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	addr, err := ngtypes.NewAddressFromBS58(params.Address)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	balance, err := s.pow.State.GetTotalBalanceByAddress(addr)
	if err != nil {
		balance = big.NewInt(0)
	}

	res := getAddressStateReply{Balance: balance.String()}
	if account, err := s.pow.State.GetContract(addr); err == nil {
		rawAcc, err := rlp.EncodeToBytes(account)
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		res.Contract = hex.EncodeToString(rawAcc)
	}
	res.Exists = balance.Sign() > 0 || res.Contract != ""

	return reply(msg, res)
}

func (s *Server) getSheetFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var sheet *ngtypes.Sheet
	err := s.pow.State.View(func(txn *bbolt.Tx) error {
		var err error
		sheet, err = ngstate.DumpSheetTxn(s.pow.State.Network, txn)
		return err
	})
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	rawSheet, err := rlp.EncodeToBytes(sheet)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, getSheetReply{
		Network:   uint8(sheet.Network),
		Height:    sheet.Height,
		BlockHash: hex.EncodeToString(sheet.BlockHash),
		Timestamp: s.pow.Chain.GetLatestBlock().GetTimestamp(),
		Sheet:     hex.EncodeToString(rawSheet),
	})
}

type addressParams struct {
	Address string `json:"address"`
}

func (s *Server) getContractInfoFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params addressParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	addr, err := ngtypes.NewAddressFromBS58(params.Address)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	account, err := s.pow.State.GetContract(addr)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, account)
}

type balanceReply struct {
	TotalBalance  string
	MatureBalance string
	LockedBalance string
}

func (s *Server) getBalanceByAddressFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params addressParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	addr, err := ngtypes.NewAddressFromBS58(params.Address)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	totalBalance, err := s.pow.State.GetTotalBalanceByAddress(addr)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	matureBalance, err := s.pow.State.GetMatureBalanceByAddress(addr)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, balanceReply{
		TotalBalance:  totalBalance.String(),
		MatureBalance: matureBalance.String(),
		LockedBalance: new(big.Int).Sub(totalBalance, matureBalance).String(),
	})
}
