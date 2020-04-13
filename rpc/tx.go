package rpc

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/maoxs2/go-jsonrpc2"
	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type sendTransactionParams struct {
	Convener     uint64        `json:"convener"`
	Participants []interface{} `json:"participants"`
	Values       []float64     `json:"values"`
	Fee          float64       `json:"fee"`
	Extra        string        `json:"extra"`
}

func (s *Server) sendTransactionFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
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
			account, err := s.sheetManager.GetAccountByID(accountID)
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

	nonce := s.sheetManager.GetNextNonce(params.Convener)

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TX_TRANSACTION,
		params.Convener,
		participants,
		values,
		fee,
		nonce,
		extra,
	)

	err = tx.Signature(s.consensus.PrivateKey)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.txPool.PutNewTxFromLocal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	result := map[string]interface{}{
		"tx": tx.BS58(),
	}
	raw, err := utils.JSON.Marshal(result)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type sendRegisterParams struct {
	ID uint64 `json:"id"`
}

func (s *Server) sendRegisterFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendRegisterParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	nonce := s.sheetManager.GetNextNonce(1)

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TX_REGISTER,
		1,
		[][]byte{
			utils.PublicKey2Bytes(*s.consensus.PrivateKey.PubKey()),
		},
		[]*big.Int{ngtypes.GetBig0()},
		new(big.Int).Mul(ngtypes.NG, big.NewInt(10)),
		nonce,
		utils.PackUint64LE(params.ID),
	)

	err = tx.Signature(s.consensus.PrivateKey)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	log.Info(new(jsonpb.Marshaler).MarshalToString(tx))

	err = s.txPool.PutNewTxFromLocal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	result := map[string]interface{}{
		"tx": tx.BS58(),
	}
	raw, err := utils.JSON.Marshal(result)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type sendLogoutParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
	Extra    string  `json:"extra"`
}

func (s *Server) sendLogoutFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendLogoutParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	nonce := s.sheetManager.GetNextNonce(params.Convener)

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TX_LOGOUT,
		params.Convener,
		nil,
		nil,
		fee,
		nonce,
		extra,
	)

	err = tx.Signature(s.consensus.PrivateKey)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.txPool.PutNewTxFromLocal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	result := map[string]interface{}{
		"tx": tx.BS58(),
	}
	raw, err := utils.JSON.Marshal(result)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type sendAssignParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
	Extra    string  `json:"extra"`
}

func (s *Server) sendAssignFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendAssignParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	nonce := s.sheetManager.GetNextNonce(params.Convener)

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TX_ASSIGN,
		params.Convener,
		nil,
		nil,
		fee,
		nonce,
		extra,
	)

	err = tx.Signature(s.consensus.PrivateKey)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.txPool.PutNewTxFromLocal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	result := map[string]interface{}{
		"tx": tx.BS58(),
	}
	raw, err := utils.JSON.Marshal(result)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type sendAppendParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
	Extra    string  `json:"extra"`
}

func (s *Server) sendAppendFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendAppendParams
	err := utils.JSON.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	nonce := s.sheetManager.GetNextNonce(params.Convener)

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		ngtypes.TX_ASSIGN,
		params.Convener,
		nil,
		nil,
		fee,
		nonce,
		extra,
	)

	err = tx.Signature(s.consensus.PrivateKey)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.txPool.PutNewTxFromLocal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	result := map[string]interface{}{
		"tx": tx.BS58(),
	}
	raw, err := utils.JSON.Marshal(result)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
