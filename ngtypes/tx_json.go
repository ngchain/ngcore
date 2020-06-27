package ngtypes

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/mr-tron/base58/base58"
	"github.com/ngchain/ngcore/utils"
)

type jsonTx struct {
	Network       int      `json:"network"`
	Type          int      `json:"type"`
	PrevBlockHash []byte   `json:"prev_block_hash"`
	Convener      uint64   `json:"convener"`
	Participants  []string `json:"participants"`
	Fee           string   `json:"fee"`
	Values        []string `json:"values"`
	Extra         string   `json:"extra"`

	Sign string `json:"sign"`

	// helpers
	Hash string `json:"hash"`
}

func (x *Tx) MarshalJSON() ([]byte, error) {
	participants := make([]string, len(x.Participants))
	for i := range x.Participants {
		participants[i] = base58.FastBase58Encoding(x.Participants[i])
	}

	values := make([]string, len(x.Values))
	for i := range x.Values {
		values[i] = new(big.Int).SetBytes(x.Values[i]).String()
	}

	return utils.JSON.Marshal(jsonTx{
		Network:       int(x.Network),
		Type:          int(x.GetType()),
		PrevBlockHash: x.PrevBlockHash,
		Convener:      x.Convener,
		Participants:  participants,
		Fee:           new(big.Int).SetBytes(x.GetFee()).String(),
		Values:        values,
		Extra:         hex.EncodeToString(x.GetExtra()),

		Sign: hex.EncodeToString(x.GetSign()),

		Hash: hex.EncodeToString(x.Hash()),
	})
}

func (x *Tx) UnmarshalJSON(b []byte) error {
	var tx jsonTx
	err := utils.JSON.Unmarshal(b, &tx)
	if err != nil {
		return err
	}

	x.Network = NetworkType(tx.Network)

	x.Type = TxType(tx.Type)
	x.Convener = uint64(tx.Convener)

	x.Participants = make([][]byte, len(tx.Participants))
	for i := range tx.Participants {
		raw, err := base58.FastBase58Decoding(tx.Participants[i])
		if err != nil {
			x.Participants = nil
			return err
		}
		x.Participants[i] = raw
	}

	bigFee, ok := new(big.Int).SetString(tx.Fee, 10)
	if !ok {
		return fmt.Errorf("failed to parse txHeader's fee")
	}
	x.Fee = bigFee.Bytes()

	x.Values = make([][]byte, len(tx.Values))
	for i := range tx.Values {
		bigV, ok := new(big.Int).SetString(tx.Values[i], 10)
		if !ok {
			x.Values = nil
			return fmt.Errorf("failed to parse txHeader's values")
		}
		x.Values[i] = bigV.Bytes()
	}

	sign, err := hex.DecodeString(tx.Sign)
	if err != nil {
		return err
	}

	x.Sign = sign
	return nil
}
