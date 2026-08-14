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
			bs58, err := base58.FastBase58Decoding(p)
			if err != nil {
				log.Error(err)
				return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
			}
			participants[i] = new(ngtypes.Address).SetBytes(bs58)
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
		s.pow.Chain.GetLatestBlockHeight()+1,
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
		s.pow.Chain.GetLatestBlockHeight()+1,
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
		s.pow.Chain.GetLatestBlockHeight()+1,
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

type editHunk struct {
	Pos uint64 `json:"pos"`
	Del string `json:"del"`
	Ins string `json:"ins"`
}

type genEditParams struct {
	Convener uint64     `json:"convener"`
	Fee      float64    `json:"fee"`
	Hunks    []editHunk `json:"hunks"`
}

// genEditFunc composes an unsigned edit tx from explicit hunks
// (del/ins are the plain contract text pieces)
func (s *Server) genEditFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genEditParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	account, err := s.pow.State.GetAccountByNum(params.Convener)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	hunks := make([]ngtypes.Hunk, len(params.Hunks))
	for i, h := range params.Hunks {
		hunks[i] = ngtypes.Hunk{Pos: h.Pos, Del: []byte(h.Del), Ins: []byte(h.Ins)}
	}

	return s.buildEditTx(msg, params.Convener, params.Fee, account.Contract, hunks)
}

type genContractUpdateParams struct {
	Convener    uint64  `json:"convener"`
	Fee         float64 `json:"fee"`
	NewContract string  `json:"newContract"`
}

// genContractUpdateFunc diffs the on-chain contract text against
// newContract and composes an unsigned edit tx carrying the minimal patch
func (s *Server) genContractUpdateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genContractUpdateParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	account, err := s.pow.State.GetAccountByNum(params.Convener)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	hunks := ngtypes.DiffHunks(account.Contract, []byte(params.NewContract))
	if len(hunks) == 0 {
		err := errors.New("new contract is identical to the on-chain one")
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return s.buildEditTx(msg, params.Convener, params.Fee, account.Contract, hunks)
}

func (s *Server) buildEditTx(msg *jsonrpc2.JsonRpcMessage, convener uint64, feeNG float64, baseText []byte, hunks []ngtypes.Hunk) *jsonrpc2.JsonRpcMessage {
	fee := new(big.Int).SetUint64(uint64(feeNG * ngtypes.FloatNG))

	rawExtra, err := ngtypes.NewEditExtra(baseText, hunks).Encode()
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		ngtypes.EditTx,
		s.pow.Chain.GetLatestBlockHeight()+1,
		ngtypes.AccountNum(convener),
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

type genLockParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
	Name     string  `json:"name"` // optional: registers <owner>.<name>
}

// genLockFunc composes an unsigned lock tx, optionally registering the
// contract's addr.name handle
func (s *Server) genLockFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genLockParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return s.buildSimpleTx(msg, ngtypes.LockTx, params.Convener, params.Fee, []byte(params.Name))
}

type genUnlockParams struct {
	Convener uint64  `json:"convener"`
	Fee      float64 `json:"fee"`
}

// genUnlockFunc composes an unsigned unlock tx
func (s *Server) genUnlockFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params genUnlockParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return s.buildSimpleTx(msg, ngtypes.UnlockTx, params.Convener, params.Fee, nil)
}

// buildSimpleTx composes an unsigned convener-only tx of the given type
func (s *Server) buildSimpleTx(msg *jsonrpc2.JsonRpcMessage, txType ngtypes.TxType, convener uint64, feeNG float64, extra []byte) *jsonrpc2.JsonRpcMessage {
	fee := new(big.Int).SetUint64(uint64(feeNG * ngtypes.FloatNG))

	tx := ngtypes.NewUnsignedTx(
		s.pow.Network,
		txType,
		s.pow.Chain.GetLatestBlockHeight()+1,
		ngtypes.AccountNum(convener),
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

type resolveContractParams struct {
	Deployer string `json:"deployer"` // bs58 address
	Name     string `json:"name"`
}

// resolveContractFunc resolves a deployer.name handle to its account num
func (s *Server) resolveContractFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params resolveContractParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	deployer, err := ngtypes.NewAddressFromBS58(params.Deployer)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	num, err := s.pow.State.ResolveContractName(deployer, params.Name)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(num)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

type getContractParams struct {
	Num uint64 `json:"num"`
}

// getContractFunc returns the on-chain contract text of the account
func (s *Server) getContractFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getContractParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	account, err := s.pow.State.GetAccountByNum(params.Num)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	raw, err := utils.JSON.Marshal(string(account.Contract))
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
