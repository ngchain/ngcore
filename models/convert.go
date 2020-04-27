// Package models is generated for swagger
package models

import (
	"encoding/hex"
	"math/big"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
)

func (tx *Tx) T() (*ngtypes.Tx, error) {
	var err error

	participants := make([][]byte, len(tx.Header.Participants))
	for i, v := range tx.Header.Participants {
		participants[i], err = base58.FastBase58Decoding(v)
		if err != nil {
			return nil, err
		}
	}

	values := make([][]byte, len(tx.Header.Values))
	for i, v := range tx.Header.Values {
		values[i] = new(big.Int).SetUint64(uint64(v * ngtypes.FloatNG)).Bytes()
	}

	extra, err := hex.DecodeString(tx.Header.Extra)
	if err != nil {
		return nil, err
	}

	sign, err := hex.DecodeString(tx.Sign)
	if err != nil {
		return nil, err
	}

	return &ngtypes.Tx{
		Network: int32(tx.Network),
		Header: &ngtypes.TxHeader{
			Type:         ngtypes.TxType(tx.Header.Type),
			Convener:     uint64(tx.Header.Convener),
			Participants: participants,
			Fee:          new(big.Int).SetUint64(uint64(tx.Header.Fee * ngtypes.FloatNG)).Bytes(),
			Values:       values,
			Nonce:        uint64(tx.Header.Nonce),
			Extra:        extra,
		},
		Sign: sign,
	}, nil
}
