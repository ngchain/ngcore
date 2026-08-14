package jsonrpc

import (
	"encoding/hex"
	"math/big"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/mr-tron/base58"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type callContractParams struct {
	// Contract is the deployer's bs58 address
	Contract string `json:"contract"`
	// Value is the NG amount the simulated tx pays to the contract
	Value float64 `json:"value"`
	// Entry optionally names the export to run (eth-style selector)
	Entry string `json:"entry"`
	// Extra is the raw args the contract reads through tx.get_extra
	Extra string `json:"extra"`
}

type jsonEvent struct {
	Contract string `json:"contract"` // bs58 address
	Topic    string `json:"topic"`
	Data     string `json:"data"` // hex
}

type callContractResult struct {
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	GasUsed uint64      `json:"gasUsed"`
	Events  []jsonEvent `json:"events,omitempty"`
}

func eventsToJSON(events []ngstate.Event) []jsonEvent {
	if len(events) == 0 {
		return nil
	}

	out := make([]jsonEvent, len(events))
	for i, e := range events {
		out[i] = jsonEvent{
			Contract: base58.FastBase58Encoding(e.Contract),
			Topic:    e.Topic,
			Data:     hex.EncodeToString(e.Data),
		}
	}

	return out
}

// callContractFunc dry-runs a contract's main against the CURRENT state:
// the journal is never flushed, so nothing changes on chain — a free
// preview of what a real transact tx would do
func (s *Server) callContractFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params callContractParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	contractAddr, err := ngtypes.NewAddressFromBS58(params.Contract)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	value := new(big.Int).SetUint64(uint64(params.Value * ngtypes.FloatNG))

	result := &callContractResult{}
	err = s.pow.State.View(func(txn *bbolt.Tx) error {
		account, err := s.pow.State.GetContract(contractAddr)
		if err != nil {
			return err
		}

		fakeTx := ngtypes.NewUnsignedTx(
			s.pow.Network,
			ngtypes.TransactTx,
			s.pow.Chain.GetLatestBlockHeight()+1,
			account.Owner,
			value,
			big.NewInt(0),
			ngtypes.EncodeCallData(params.Entry, []byte(params.Extra)),
		)

		latest := s.pow.Chain.GetLatestBlock().(*ngtypes.FullBlock)

		vm, err := ngstate.NewVM(txn, account, fakeTx, latest.BlockHeader.Timestamp)
		if err != nil {
			return err
		}

		gasUsed, runErr := vm.DryRun(vm.EntryFor(ngstate.VMEntryOnTx))
		result.GasUsed = gasUsed
		if runErr != nil {
			result.Error = runErr.Error()
		} else {
			result.Success = true
			result.Events = eventsToJSON(vm.Events())
		}

		return nil
	})
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(result)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getReceiptParams struct {
	Hash string `json:"hash"` // hex tx hash
}

type jsonContractRun struct {
	Contract string      `json:"contract"` // bs58 address
	Entry    string      `json:"entry"`
	Success  bool        `json:"success"`
	Error    string      `json:"error,omitempty"`
	GasUsed  uint64      `json:"gasUsed"`
	Events   []jsonEvent `json:"events,omitempty"`
}

type getReceiptResult struct {
	OnChain       bool              `json:"onChain"`
	BlockHash     string            `json:"blockHash,omitempty"`
	BlockHeight   uint64            `json:"blockHeight,omitempty"`
	Confirmations uint64            `json:"confirmations,omitempty"`
	Runs          []jsonContractRun `json:"runs"`
}

// getReceiptFunc returns the LOCAL execution receipt of a tx: which
// contracts ran, their outcome, gas and events. Receipts are derived by
// executing the chain, not consensus data
func (s *Server) getReceiptFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getReceiptParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	txHash, err := hex.DecodeString(params.Hash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	result := &getReceiptResult{Runs: []jsonContractRun{}}

	if blockHash, height, err := s.pow.Chain.GetTxLocation(txHash); err == nil {
		result.OnChain = true
		result.BlockHash = hex.EncodeToString(blockHash)
		result.BlockHeight = height
		if latest := s.pow.Chain.GetLatestBlockHeight(); latest >= height {
			result.Confirmations = latest - height + 1
		}
	}

	runs, err := s.pow.State.GetTxRuns(txHash)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	for _, run := range runs {
		result.Runs = append(result.Runs, jsonContractRun{
			Contract: base58.FastBase58Encoding(run.Contract),
			Entry:    run.Entry,
			Success:  run.Ok,
			Error:    run.Error,
			GasUsed:  run.GasUsed,
			Events:   eventsToJSON(run.Events),
		})
	}

	raw, err := utils.JSON.Marshal(result)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
