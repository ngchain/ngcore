package jsonrpc

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"

	"github.com/ngchain/ngcore/storage"

	"github.com/maoxs2/go-jsonrpc2"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
	"github.com/ngchain/secp256k1"
)

type sendTxParams struct {
	RawTx string `json:"rawTx"`
	// add some more opinions
}

func (s *Server) sendTxFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendTxParams

	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	signedTxRaw, err := hex.DecodeString(params.RawTx)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := &ngtypes.Tx{}
	err = utils.Proto.Unmarshal(signedTxRaw, tx)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = ngstate.GetTxPool().PutNewTxFromLocal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(hex.EncodeToString(tx.Hash()))
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type signTxParams struct {
	RawTx       string   `json:"rawTx"`
	PrivateKeys []string `json:"privateKeys"`
}

// signTxFunc receives the Proto encoded bytes of unsigned Tx and return the Proto encoded bytes of signed Tx
func (s *Server) signTxFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params signTxParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	unsignedTxRaw, err := hex.DecodeString(params.RawTx)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := &ngtypes.Tx{}
	err = utils.Proto.Unmarshal(unsignedTxRaw, tx)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	privateKeys := make([]*secp256k1.PrivateKey, len(params.PrivateKeys))
	for i := range params.PrivateKeys {
		d, err := base58.FastBase58Decoding(params.PrivateKeys[i])
		if err != nil {
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}

		privateKeys[i] = secp256k1.NewPrivateKey(new(big.Int).SetBytes(d))
	}

	err = tx.Signature(privateKeys...)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	rawTx, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(hex.EncodeToString(rawTx))
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type genTransactionParams struct {
	Convener     uint64        `json:"convener"`
	Participants []interface{} `json:"participants"`
	Values       []float64     `json:"values"`
	Fee          float64       `json:"fee"`
	Extra        string        `json:"extra"`
}

// all genTx should reply protobuf encoded bytes
func (s *Server) genTransactionFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genTransactionParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var participants = make([][]byte, len(params.Participants))
	for i := range params.Participants {
		switch p := params.Participants[i].(type) {
		case string:
			participants[i], err = base58.FastBase58Decoding(p)
			if err != nil {
				return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
			}
		case float64:
			accountID := uint64(p)
			account, err := ngstate.GetCurrentState().GetAccountByNum(accountID)
			if err != nil {
				return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
			}
			participants[i] = account.Owner
		default:
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, fmt.Errorf("unknown participant type: %s", reflect.TypeOf(p))))

		}
	}

	var values = make([]*big.Int, len(params.Values))
	for i := range params.Values {
		values[i] = new(big.Int).SetUint64(uint64(params.Values[i] * ngtypes.FloatNG))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TxType_TRANSACTION,
		storage.GetChain().GetLatestBlockHash(),
		params.Convener,
		participants,
		values,
		fee,
		extra,
	)

	// providing Proto encoded bytes
	// Reason: 1. avoid accident client modification 2. less length
	rawTx, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(hex.EncodeToString(rawTx))
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type genRegisterParams struct {
	Owner ngtypes.Address `json:"owner"`
	Num   uint64          `json:"num"`
}

func (s *Server) genRegisterFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genRegisterParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TxType_REGISTER,
		storage.GetChain().GetLatestBlockHash(),
		1,
		[][]byte{
			params.Owner,
		},
		[]*big.Int{ngtypes.GetBig0()},
		new(big.Int).Mul(ngtypes.NG, big.NewInt(10)),
		utils.PackUint64LE(params.Num),
	)

	rawTx, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(hex.EncodeToString(rawTx))
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type genLogoutParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
	Extra    string  `json:"extra"`
}

func (s *Server) genLogoutFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genLogoutParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TxType_LOGOUT,
		storage.GetChain().GetLatestBlockHash(),
		params.Convener,
		nil,
		nil,
		fee,
		extra,
	)

	rawTx, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(hex.EncodeToString(rawTx))
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type genAssignParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
	Extra    string  `json:"extra"`
}

func (s *Server) genAssignFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genAssignParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TxType_ASSIGN,
		storage.GetChain().GetLatestBlockHash(),
		params.Convener,
		nil,
		nil,
		fee,
		extra,
	)

	rawTx, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(hex.EncodeToString(rawTx))
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type genAppendParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
	Extra    string  `json:"extra"`
}

func (s *Server) genAppendFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genAppendParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TxType_APPEND,
		storage.GetChain().GetLatestBlockHash(),
		params.Convener,
		nil,
		nil,
		fee,
		extra,
	)

	rawTx, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(hex.EncodeToString(rawTx))
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
