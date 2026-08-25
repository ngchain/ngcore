package jsonrpc

import (
	"encoding/hex"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type sendTxParams struct {
	RawTx string `json:"rawTx"`
	// add some more opinions
}

func (s *Server) sendTxFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendTxParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	signedTxRaw, err := hex.DecodeString(params.RawTx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var tx ngtypes.FullTx
	err = rlp.DecodeBytes(signedTxRaw, &tx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.pow.Pool.PutNewTxFromLocal(&tx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, hex.EncodeToString(tx.GetHash()))
}

type sendCommitmentParams struct {
	RawCommitment string `json:"rawCommitment"`
}

// sendCommitmentFunc pools and broadcasts a signed commitment: the blind half
// of the mandatory commit-reveal flow. The wallet signs a Commitment over
// blake3(revealTx.UnheightedHash() ‖ salt) and submits it here; once it is on
// chain, the matching reveal tx (carrying the same Salt) becomes admissible
// via ng_sendTx in a LATER block.
func (s *Server) sendCommitmentFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendCommitmentParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	signedRaw, err := hex.DecodeString(params.RawCommitment)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var commit ngtypes.Commitment
	err = rlp.DecodeBytes(signedRaw, &commit)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.pow.Pool.PutNewCommitmentFromLocal(&commit)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, hex.EncodeToString(commit.Hash))
}

type genTransactionParams struct {
	To    string `json:"to"`    // bs58 address
	Value string `json:"value"` // decimal NG ("1.5"); exact
	Fee   string `json:"fee"`
	Entry string `json:"entry"` // optional contract export name (empty = main)
	Extra string `json:"extra"` // hex args
}

// all genTx should reply protobuf encoded bytes.
func (s *Server) genTransactionFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genTransactionParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	to, err := ngtypes.NewAddressFromBS58(params.To)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	value, err := ngAmount(params.Value)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	fee, err := ngAmount(params.Fee)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	args, err := hex.DecodeString(params.Extra)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	extra := ngtypes.EncodeCallData(params.Entry, args)

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		ngtypes.TransactTx,
		s.pow.Chain.GetLatestBlockHeight()+1,
		to,
		value,
		fee,
		extra,
	)

	// providing Proto encoded bytes
	// Reason: 1. avoid accident client modification 2. less length
	rawTx, err := rlp.EncodeToBytes(tx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, hex.EncodeToString(rawTx))
}

type genCommitParams struct {
	Fee  string `json:"fee"`  // decimal NG; exact
	Wasm string `json:"wasm"` // hex of the compiled contract module
}

// genDeployFunc composes an unsigned deploy tx carrying the WHOLE
// compiled module (compressed). The first deploy on an address opens its
// contract slot and goes LIVE at once; a later deploy replaces the code
// wholesale, UUPS-style (the current code must export `upgrade`)
func (s *Server) genDeployFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genCommitParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	module, err := hex.DecodeString(params.Wasm)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return s.buildSimpleTx(msg, ngtypes.DeployTx, params.Fee, ngtypes.EncodeCommitCode(module))
}

// buildSimpleTx composes an unsigned from-only tx of the given type
func (s *Server) buildSimpleTx(msg *jsonrpc2.JsonRpcMessage, txType ngtypes.TxType, feeNG string, extra []byte) *jsonrpc2.JsonRpcMessage {
	fee, err := ngAmount(feeNG)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		txType,
		s.pow.Chain.GetLatestBlockHeight()+1,
		ngtypes.Address{},
		nil,
		fee,
		extra,
	)

	rawTx, err := rlp.EncodeToBytes(tx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, hex.EncodeToString(rawTx))
}
