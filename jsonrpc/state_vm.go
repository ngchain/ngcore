package jsonrpc

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"

	"github.com/c0mm4nd/go-jsonrpc2"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type callContractParams struct {
	// Contract identifies the target: "700" or "<deployerBS58>.<name>"
	Contract string `json:"contract"`
	// Caller poses as the tx convener (default 1)
	Caller uint64 `json:"caller"`
	// Value is the NG amount the simulated tx pays to the contract
	Value float64 `json:"value"`
	// Extra is the simulated tx extra (the calldata channel)
	Extra string `json:"extra"`
}

type jsonEvent struct {
	Contract uint64 `json:"contract"`
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
			Contract: e.Contract,
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

	num, err := s.resolveContractRef(params.Contract)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	caller := params.Caller
	if caller == 0 {
		caller = 1
	}
	value := new(big.Int).SetUint64(uint64(params.Value * ngtypes.FloatNG))

	result := &callContractResult{}
	err = s.pow.State.View(func(txn *bbolt.Tx) error {
		account, err := s.pow.State.GetAccountByNum(num)
		if err != nil {
			return err
		}

		fakeTx := ngtypes.NewUnsignedTx(
			s.pow.Network,
			ngtypes.TransactTx,
			s.pow.Chain.GetLatestBlockHeight()+1,
			ngtypes.AccountNum(caller),
			[]ngtypes.Address{account.Owner},
			[]*big.Int{value},
			big.NewInt(0),
			[]byte(params.Extra),
		)

		latest := s.pow.Chain.GetLatestBlock().(*ngtypes.FullBlock)

		vm, err := ngstate.NewVM(txn, account, fakeTx, latest.BlockHeader.Timestamp)
		if err != nil {
			return err
		}

		gasUsed, runErr := vm.DryRun(ngstate.VMEntryOnTx)
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

// resolveContractRef accepts "700" or "<deployerBS58>.<name>"
func (s *Server) resolveContractRef(ref string) (uint64, error) {
	if dot := strings.IndexByte(ref, '.'); dot >= 0 {
		deployer, err := ngtypes.NewAddressFromBS58(ref[:dot])
		if err != nil {
			return 0, err
		}

		return s.pow.State.ResolveContractName(deployer, ref[dot+1:])
	}

	return strconv.ParseUint(ref, 10, 64)
}

type getReceiptParams struct {
	Hash string `json:"hash"` // hex tx hash
}

type jsonContractRun struct {
	Account uint64      `json:"account"`
	Entry   string      `json:"entry"`
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	GasUsed uint64      `json:"gasUsed"`
	Events  []jsonEvent `json:"events,omitempty"`
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
			Account: run.Account,
			Entry:   run.Entry,
			Success: run.Ok,
			Error:   run.Error,
			GasUsed: run.GasUsed,
			Events:  eventsToJSON(run.Events),
		})
	}

	raw, err := utils.JSON.Marshal(result)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
