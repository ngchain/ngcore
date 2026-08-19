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

// ngToRaw converts a float64 amount of whole NG (the human-facing rpc
// unit) into raw 18-decimal units. Via big.Float at 256-bit precision:
// `uint64(v * 1e18)` overflows for anything above ~18.4 NG (the u64
// ceiling), silently corrupting large transfers — and the default
// 53-bit big.Float would still round large products. The only remaining
// imprecision is the caller's own float64 (exactly-representable values
// convert exactly)
func ngToRaw(v float64) *big.Int {
	scale := new(big.Float).SetPrec(256).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	amount := new(big.Float).SetPrec(256).SetFloat64(v)
	raw, _ := amount.Mul(amount, scale).Int(nil)
	if raw == nil || raw.Sign() < 0 {
		return big.NewInt(0)
	}
	return raw
}
