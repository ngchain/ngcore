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

	privateKeys := make([]*ngtypes.PrivateKey, len(params.PrivateKeys))
	for i := range params.PrivateKeys {
		d, err := base58.FastBase58Decoding(params.PrivateKeys[i])
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}

		privateKeys[i], err = ngtypes.ParsePrivateKey(d)
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
	}

	err = tx.Signature(privateKeys...)
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
	Participants []string  `json:"participants"` // bs58 addresses
	Values       []float64 `json:"values"`
	Fee          float64   `json:"fee"`
	Entry        string    `json:"entry"` // optional contract entry (eth-style selector)
	Extra        string    `json:"extra"` // hex args
}

// all genTx should reply protobuf encoded bytes.
func (s *Server) genTransactionFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genTransactionParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	participants := make([]ngtypes.Address, len(params.Participants))
	for i := range params.Participants {
		addr, err := ngtypes.NewAddressFromBS58(params.Participants[i])
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		participants[i] = addr
	}

	values := make([]*big.Int, len(params.Values))
	for i := range params.Values {
		values[i] = new(big.Int).SetUint64(uint64(params.Values[i] * ngtypes.FloatNG))
	}

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
		participants,
		values,
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
		nil,
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

type commitHunk struct {
	Pos uint64 `json:"pos"`
	Del string `json:"del"`
	Ins string `json:"ins"`
}

type genCommitParams struct {
	Address string       `json:"address"` // the deployer's own address (its slot text is the base)
	Fee     float64      `json:"fee"`
	Hunks   []commitHunk `json:"hunks"`
}

// genCommitFunc composes an unsigned commit tx from explicit hunks
// (del/ins are the plain contract text pieces)
func (s *Server) genCommitFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genCommitParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	baseText := s.slotText(params.Address)

	hunks := make([]ngtypes.Hunk, len(params.Hunks))
	for i, h := range params.Hunks {
		hunks[i] = ngtypes.Hunk{Pos: h.Pos, Del: []byte(h.Del), Ins: []byte(h.Ins)}
	}

	return s.buildCommitTx(msg, params.Fee, baseText, hunks)
}

type genContractUpdateParams struct {
	Address     string  `json:"address"` // the deployer's own address
	Fee         float64 `json:"fee"`
	NewContract string  `json:"newContract"`
}

// genContractUpdateFunc diffs the on-chain contract text against
// newContract and composes an unsigned commit tx carrying the minimal patch
func (s *Server) genContractUpdateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genContractUpdateParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	baseText := s.slotText(params.Address)

	hunks := ngtypes.DiffHunks(baseText, []byte(params.NewContract))
	if len(hunks) == 0 {
		err := errors.New("new contract is identical to the on-chain one")
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return s.buildCommitTx(msg, params.Fee, baseText, hunks)
}

// slotText loads the current contract text of the address; empty when
// the slot never opened (the edit will then be the deploy)
func (s *Server) slotText(address string) []byte {
	addr, err := ngtypes.NewAddressFromBS58(address)
	if err != nil {
		return nil
	}

	account, err := s.pow.State.GetContract(addr)
	if err != nil {
		return nil
	}

	return account.Source
}

func (s *Server) buildCommitTx(msg *jsonrpc2.JsonRpcMessage, feeNG float64, baseText []byte, hunks []ngtypes.Hunk) *jsonrpc2.JsonRpcMessage {
	fee := new(big.Int).SetUint64(uint64(feeNG * ngtypes.FloatNG))

	rawExtra, err := ngtypes.NewCommitExtra(baseText, hunks).Encode()
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		ngtypes.CommitTx,
		s.pow.Chain.GetLatestBlockHeight()+1,
		nil,
		nil,
		fee,
		rawExtra,
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

type genActivateParams struct {
	Fee float64 `json:"fee"`
}

// genActivateFunc composes an unsigned activate tx (the sender locks its own
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

// buildSimpleTx composes an unsigned sender-only tx of the given type
func (s *Server) buildSimpleTx(msg *jsonrpc2.JsonRpcMessage, txType ngtypes.TxType, feeNG float64, extra []byte) *jsonrpc2.JsonRpcMessage {
	fee := new(big.Int).SetUint64(uint64(feeNG * ngtypes.FloatNG))

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		txType,
		s.pow.Chain.GetLatestBlockHeight()+1,
		nil,
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

// getContractFunc returns the on-chain contract text of the address
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

	raw, err := utils.JSON.Marshal(string(account.Source))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
