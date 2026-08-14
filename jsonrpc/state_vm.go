package jsonrpc

import (
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

type callContractResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	GasUsed uint64 `json:"gasUsed"`
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
