package jsonrpc

import (
	"math/big"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// some utils for wallet clients

type getAddressParams struct {
	PrivateKeys []string
}

type getAddressReply struct {
	Address ngtypes.Address
}

// publicKeyToAddressFunc helps client to get the schnorr publickey of private keys.
func (s *Server) publicKeyToAddressFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getAddressParams
	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	if len(params.PrivateKeys) != 1 {
		err := errors.New("expects exactly one private key")
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	bPriv, err := base58.FastBase58Decoding(params.PrivateKeys[0])
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	privKey, err := ngtypes.ParsePrivateKey(bPriv)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	addr := ngtypes.NewAddress(privKey)

	result := getAddressReply{
		Address: addr,
	}

	raw, err := utils.JSON.Marshal(result)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}

// ngAmount parses a decimal string of whole NG ("1.5") into raw
// 18-decimal units, EXACTLY — no float anywhere on a money path. Empty
// means zero; more than 18 fractional digits, negatives and garbage are
// rejected
func ngAmount(s string) (*big.Int, error) {
	if s == "" {
		return big.NewInt(0), nil
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, errors.Errorf("bad NG amount %q", s)
	}
	r.Mul(r, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)))
	if !r.IsInt() {
		return nil, errors.Errorf("NG amount %q has more than 18 decimal places", s)
	}
	raw := r.Num()
	if raw.Sign() < 0 {
		return nil, errors.Errorf("NG amount %q is negative", s)
	}
	return raw, nil
}

// reply marshals v as the success result of msg, or returns a json-rpc
// error if marshaling fails. Collapses the marshal-and-guard boilerplate
// that was copy-pasted into ~24 handlers (each an unreachable error arm)
// into one place — the reply types never fail to marshal
func reply(msg *jsonrpc2.JsonRpcMessage, v interface{}) *jsonrpc2.JsonRpcMessage {
	raw, err := utils.JSON.Marshal(v)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	return jsonrpc2.NewJsonRpcSuccess(msg.ID, raw)
}
