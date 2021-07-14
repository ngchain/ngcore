package jsonrpc

import (
	"encoding/hex"
	"math/big"
	"reflect"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/rlp"
	"github.com/mr-tron/base58"
	"github.com/ngchain/secp256k1"
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

	var tx ngtypes.Tx
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

	var tx ngtypes.Tx
	err = rlp.DecodeBytes(unsignedTxRaw, &tx)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	privateKeys := make([]*secp256k1.PrivateKey, len(params.PrivateKeys))
	for i := range params.PrivateKeys {
		d, err := base58.FastBase58Decoding(params.PrivateKeys[i])
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}

		privateKeys[i] = secp256k1.NewPrivateKey(new(big.Int).SetBytes(d))
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
	Convener     uint64        `json:"convener"`
	Participants []interface{} `json:"participants"`
	Values       []float64     `json:"values"`
	Fee          float64       `json:"fee"`
	Extra        string        `json:"extra"`
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
		switch p := params.Participants[i].(type) {
		case string:
			participants[i], err = base58.FastBase58Decoding(p)
			if err != nil {
				log.Error(err)
				return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
			}
		case float64:
			accountID := uint64(p)
			account, err := s.pow.State.GetAccountByNum(accountID)
			if err != nil {
				log.Error(err)
				return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
			}
			participants[i] = account.Owner
		default:
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0,
				errors.Wrapf(ngtypes.ErrTxParticipantsInvalid, "unknown participant type: %s", reflect.TypeOf(p))))
		}
	}

	values := make([]*big.Int, len(params.Values))
	for i := range params.Values {
		values[i] = new(big.Int).SetUint64(uint64(params.Values[i] * ngtypes.FloatNG))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	extra, err := hex.DecodeString(params.Extra)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		ngtypes.TransactTx,
		s.pow.Chain.GetLatestBlockHeight(),
		ngtypes.AccountNum(params.Convener),
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

type genRegisterParams struct {
	Owner ngtypes.Address `json:"owner"`
	Num   uint64          `json:"num"`
}

func (s *Server) genRegisterFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genRegisterParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		ngtypes.RegisterTx,
		s.pow.Chain.GetLatestBlockHeight(),
		1,
		[]ngtypes.Address{
			params.Owner,
		},
		[]*big.Int{big.NewInt(0)},
		new(big.Int).Mul(ngtypes.NG, big.NewInt(10)),
		utils.PackUint64LE(params.Num),
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

type genDestroyParams struct {
	Convener  uint64  `json:"convener"`
	Fee       float64 `json:"fee"`
	PublicKey string  `json:"publicKey"` // compressed publicKey, beginning with 02 or 03 (not 04).
}

func (s *Server) genDestroyFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genDestroyParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	extra, err := hex.DecodeString(params.PublicKey)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		ngtypes.DestroyTx,
		s.pow.Chain.GetLatestBlockHeight(),
		ngtypes.AccountNum(params.Convener),
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

type genAppendParams struct {
	Convener     uint64  `json:"convener"`
	Fee          float64 `json:"fee"`
	ExtraPos     uint64  `json:"extraPos"`
	ExtraContent string  `json:"extraContent"`
}

func (s *Server) genAppendFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genAppendParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	extraContent, err := hex.DecodeString(params.ExtraContent)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	extra := &ngtypes.DeleteExtra{
		Pos:     params.ExtraPos,
		Content: extraContent,
	}

	rawExtra, err := rlp.EncodeToBytes(extra)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		ngtypes.AppendTx,
		s.pow.Chain.GetLatestBlockHeight(),
		ngtypes.AccountNum(params.Convener),
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

type genDeleteParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`

	ExtraContent string `json:"extraContent"`
	ExtraPos     uint64 `json:"extraPos"`
}

func (s *Server) genDeleteFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genDeleteParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	extraContent, err := hex.DecodeString(params.ExtraContent)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	extra := &ngtypes.DeleteExtra{
		Pos:     params.ExtraPos,
		Content: extraContent,
	}

	rawExtra, err := rlp.EncodeToBytes(extra)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		ngtypes.DeleteTx,
		s.pow.Chain.GetLatestBlockHeight(),
		ngtypes.AccountNum(params.Convener),
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
