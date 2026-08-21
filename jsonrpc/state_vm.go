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
	// Value is the NG amount the simulated tx pays to the contract, as
	// a decimal string ("1.5") — exact, no float on any money path
	Value string `json:"value"`
	// Entry optionally names the export to run (by name; empty = main)
	Entry string `json:"entry"`
	// Extra is the raw args the contract reads through tx.get_extra
	Extra string `json:"extra"`
	// Height, when set, dry-runs against the state reconstructed as of that
	// past block (isolated scratch db); nil uses the current state
	Height *uint64 `json:"height,omitempty"`
}

// dryRunContract simulates a transact against `account` on the given txn's
// state, filling result with the outcome, gas, events and trace
func (s *Server) dryRunContract(txn *bbolt.Tx, account *ngtypes.Contract, height, blockTime uint64, value *big.Int, entry, extra string, result *callContractResult) error {
	fakeTx := ngtypes.NewUnsignedTx(
		s.pow.Network, ngtypes.TransactTx, height, account.Owner, value, big.NewInt(0),
		ngtypes.EncodeCallData(entry, []byte(extra)),
	)

	vm, err := ngstate.NewVM(txn, account, fakeTx, blockTime)
	if err != nil {
		return err
	}

	gasUsed, runErr := vm.DryRun(vm.EntryFor(ngstate.VMEntryOnTx))
	result.GasUsed = gasUsed
	result.Trace = vm.Trace() // internal call/transfer tree, kept even on failure
	if runErr != nil {
		result.Error = runErr.Error()
	} else {
		result.Ok = true
		result.Events = vm.Events()
	}
	return nil
}

// ngstate.Event / ngstate.ContractRun carry canonical MarshalJSON
// (bs58 addresses, hex bytes), reused across getReceipt, callContract and
// the trace endpoints so the run/event shape is uniform — `ok`, not
// `success`, everywhere
type callContractResult struct {
	Ok      bool                `json:"ok"`
	Error   string              `json:"error,omitempty"`
	GasUsed uint64              `json:"gasUsed"`
	Events  []ngstate.Event     `json:"events,omitempty"`
	Trace   []ngstate.TraceCall `json:"trace,omitempty"` // internal call/transfer tree of the dry-run
}

type getContractExportsParams struct {
	Address string  `json:"address"`
	Height  *uint64 `json:"height,omitempty"`
}

type contractExportReply struct {
	Name     string `json:"name"`
	Params   int    `json:"params"`
	Results  int    `json:"results"`
	Callable bool   `json:"callable"` // a transact tx can dispatch to it
}

// getContractExportsFunc lists a contract's exported functions (its "ABI"),
// marking those a transact tx can call. Optional height reads a past version
func (s *Server) getContractExportsFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getContractExportsParams
	if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	addr, err := ngtypes.NewAddressFromBS58(params.Address)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var account *ngtypes.Contract
	if params.Height != nil {
		account, err = s.pow.State.GetContractAt(addr, *params.Height)
	} else {
		account, err = s.pow.State.GetContract(addr)
	}
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	exports, err := ngstate.ContractExports(account.Source)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	out := make([]contractExportReply, len(exports))
	for i, e := range exports {
		out[i] = contractExportReply{Name: e.Name, Params: e.Params, Results: e.Results, Callable: e.Callable}
	}

	return reply(msg, out)
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

	value, err := ngAmount(params.Value)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	result := &callContractResult{}
	if params.Height != nil {
		// simulate against the state reconstructed at that past height
		block, err := s.pow.Chain.GetBlockByHeight(*params.Height)
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		err = s.pow.State.ReconstructAt(*params.Height, func(txn *bbolt.Tx) error {
			account, err := ngstate.GetContractTxn(txn, contractAddr)
			if err != nil {
				return err
			}
			return s.dryRunContract(txn, account, *params.Height+1, block.GetTimestamp(), value, params.Entry, params.Extra, result)
		})
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		return reply(msg, result)
	}

	err = s.pow.State.View(func(txn *bbolt.Tx) error {
		account, err := ngstate.GetContractTxn(txn, contractAddr)
		if err != nil {
			return err
		}
		latest := s.pow.Chain.GetLatestBlock().(*ngtypes.FullBlock)
		return s.dryRunContract(txn, account, latest.GetHeight()+1, latest.BlockHeader.Timestamp, value, params.Entry, params.Extra, result)
	})
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, result)
}

type getReceiptParams struct {
	Hash string `json:"hash"` // hex tx hash
}

type jsonContractRun struct {
	Contract string          `json:"contract"` // bs58 address
	Entry    string          `json:"entry"`
	Ok       bool            `json:"ok"`
	Error    string          `json:"error,omitempty"`
	GasUsed  uint64          `json:"gasUsed"`
	Events   []ngstate.Event `json:"events,omitempty"`
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
			Ok:       run.Ok,
			Error:    run.Error,
			GasUsed:  run.GasUsed,
			Events:   run.Events,
		})
	}

	return reply(msg, result)
}
