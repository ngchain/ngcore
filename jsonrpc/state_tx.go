package jsonrpc

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"

	"github.com/maoxs2/go-jsonrpc2"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/txpool"
	"github.com/ngchain/ngcore/utils"
	"github.com/ngchain/secp256k1"
)

type sendTxParams struct {
	SignedRawTx string `json:"signedRaw"`
}

func (s *Server) sendTxFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params signTxParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	signedTxRaw, err := hex.DecodeString(params.Raw)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := &ngtypes.Tx{}
	err = utils.Proto.Unmarshal(signedTxRaw, tx)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = txpool.GetTxPool().PutNewTxFromLocal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, []byte("OK"))
}

type signTxParams struct {
	Raw         string   `json:"raw"`
	PrivateKeys []string `json:"privateKeys"`
}

type signTxReply struct {
	SignedRawTx string
}

func (s *Server) signTxFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params signTxParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	unsignedTxRaw, err := hex.DecodeString(params.Raw)
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
		d, err := hex.DecodeString(params.PrivateKeys[i])
		if err != nil {
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}

		privateKeys[i] = secp256k1.NewPrivateKey(new(big.Int).SetBytes(d))
	}

	err = tx.Signature(privateKeys...)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type sendTransactionParams struct {
	Convener     uint64        `json:"convener"`
	Participants []interface{} `json:"participants"`
	Values       []float64     `json:"values"`
	Fee          float64       `json:"fee"`
	Extra        string        `json:"extra"`
}

func (s *Server) genTransactionFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendTransactionParams
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

	nonce := ngstate.GetCurrentState().GetNextNonce(params.Convener)

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TxType_TRANSACTION,
		params.Convener,
		participants,
		values,
		fee,
		nonce,
		extra,
	)

	raw, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, []byte(hex.EncodeToString(raw)))
}

type sendRegisterParams struct {
	ID uint64 `json:"id"`
}

func (s *Server) genRegisterFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendRegisterParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	nonce := ngstate.GetCurrentState().GetNextNonce(1)

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TxType_REGISTER,
		1,
		[][]byte{
			utils.PublicKey2Bytes(*consensus.GetPoWConsensus().PrivateKey.PubKey()),
		},
		[]*big.Int{ngtypes.GetBig0()},
		new(big.Int).Mul(ngtypes.NG, big.NewInt(10)),
		nonce,
		utils.PackUint64LE(params.ID),
	)

	raw, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, []byte(hex.EncodeToString(raw)))
}

type sendLogoutParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
	Extra    string  `json:"extra"`
}

func (s *Server) genLogoutFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendLogoutParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	nonce := ngstate.GetCurrentState().GetNextNonce(params.Convener)

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TxType_LOGOUT,
		params.Convener,
		nil,
		nil,
		fee,
		nonce,
		extra,
	)

	raw, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, []byte(hex.EncodeToString(raw)))
}

type sendAssignParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
	Extra    string  `json:"extra"`
}

func (s *Server) genAssignFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendAssignParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	nonce := ngstate.GetCurrentState().GetNextNonce(params.Convener)

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TxType_ASSIGN,
		params.Convener,
		nil,
		nil,
		fee,
		nonce,
		extra,
	)

	raw, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, []byte(hex.EncodeToString(raw)))
}

type sendAppendParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
	Extra    string  `json:"extra"`
}

func (s *Server) genAppendFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendAppendParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	nonce := ngstate.GetCurrentState().GetNextNonce(params.Convener)

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TxType_APPEND,
		params.Convener,
		nil,
		nil,
		fee,
		nonce,
		extra,
	)

	raw, err := utils.Proto.Marshal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, []byte(hex.EncodeToString(raw)))
}
