package jsonrpc

import (
	"encoding/hex"
	"math/big"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/rlp"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

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

	raw, err := utils.JSON.Marshal(hex.EncodeToString(tx.GetHash()))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type signTxParams struct {
	RawTx       string   `json:"rawTx"`
	PrivateKeys []string `json:"privateKeys"`
}

// signTxFunc receives the Proto encoded bytes of unsigned Tx and return the Proto encoded bytes of signed Tx.
func (s *Server) signTxFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params signTxParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	unsignedTxRaw, err := hex.DecodeString(params.RawTx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var tx ngtypes.FullTx
	err = rlp.DecodeBytes(unsignedTxRaw, &tx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	if len(params.PrivateKeys) != 1 {
		err := errors.New("signTx expects exactly one private key")
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	d, err := base58.FastBase58Decoding(params.PrivateKeys[0])
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	privateKey, err := ngtypes.ParsePrivateKey(d)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	// once the chain knows this key, the compact envelope saves the
	// public key bytes automatically
	if s.pow.State.PubKeyRegistered(ngtypes.NewAddress(privateKey)) {
		err = tx.SignatureCompact(privateKey)
	} else {
		err = tx.Signature(privateKey)
	}
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	rawTx, err := rlp.EncodeToBytes(tx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(hex.EncodeToString(rawTx))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type genTransactionParams struct {
	To    string  `json:"to"` // bs58 address
	Value float64 `json:"value"`
	Fee   float64 `json:"fee"`
	Entry string  `json:"entry"` // optional contract export name (empty = main)
	Extra string  `json:"extra"` // hex args
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

	value := new(big.Int).SetUint64(uint64(params.Value * ngtypes.FloatNG))
	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

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

	raw, err := utils.JSON.Marshal(hex.EncodeToString(rawTx))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type genDestroyParams struct {
	Fee   float64 `json:"fee"`
	Extra string  `json:"extra"` // optional hex payload
}

func (s *Server) genDestroyFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genDestroyParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		ngtypes.DestroyTx,
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

	raw, err := utils.JSON.Marshal(hex.EncodeToString(rawTx))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type genCommitParams struct {
	Fee  float64 `json:"fee"`
	Wasm string  `json:"wasm"` // hex of the compiled contract module
}

// genCommitFunc composes an unsigned commit tx carrying the WHOLE
// compiled module (compressed). The first commit on an address opens
// its contract slot; later commits replace the code wholesale — a
// snapshot, not a diff, since compiled wasm relayouts on any change
func (s *Server) genCommitFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
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

	return s.buildSimpleTx(msg, ngtypes.CommitTx, params.Fee, ngtypes.EncodeCommitCode(module))
}

type genActivateParams struct {
	Fee float64 `json:"fee"`
}

// genActivateFunc composes an unsigned activate tx (the From address locks its own
// contract slot)
func (s *Server) genActivateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genActivateParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return s.buildSimpleTx(msg, ngtypes.ActivateTx, params.Fee, nil)
}

type genDeactivateParams struct {
	Fee float64 `json:"fee"`
}

// genDeactivateFunc composes an unsigned unactivate tx
func (s *Server) genDeactivateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genDeactivateParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return s.buildSimpleTx(msg, ngtypes.DeactivateTx, params.Fee, nil)
}

// buildSimpleTx composes an unsigned from-only tx of the given type
func (s *Server) buildSimpleTx(msg *jsonrpc2.JsonRpcMessage, txType ngtypes.TxType, feeNG float64, extra []byte) *jsonrpc2.JsonRpcMessage {
	fee := new(big.Int).SetUint64(uint64(feeNG * ngtypes.FloatNG))

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

	raw, err := utils.JSON.Marshal(hex.EncodeToString(rawTx))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getContractParams struct {
	Address string `json:"address"`
}

// getContractFunc returns the on-chain contract wasm (hex) of the address
func (s *Server) getContractFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getContractParams
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

	raw, err := utils.JSON.Marshal(hex.EncodeToString(account.Source))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
