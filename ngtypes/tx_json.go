package ngtypes

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/mr-tron/base58/base58"
	"github.com/ngchain/ngcore/utils"
)

type jsonTxHeader struct {
	Type         int      `json:"type"`
	Convener     uint64   `json:"convener"`
	Participants []string `json:"participants"`
	Fee          string   `json:"fee"`
	Values       []string `json:"values"`
	N            uint64   `json:"n"`
	Extra        string   `json:"extra"`
}

func (x TxHeader) MarshalJSON() ([]byte, error) {
	participants := make([]string, len(x.Participants))
	for i := range x.Participants {
		participants[i] = base58.FastBase58Encoding(x.Participants[i])
	}

	values := make([]string, len(x.Values))
	for i := range x.Values {
		values[i] = new(big.Int).SetBytes(x.Values[i]).String()
	}

	return utils.JSON.Marshal(jsonTxHeader{
		Type:         int(x.GetType()),
		Convener:     x.Convener,
		Participants: participants,
		Fee:          new(big.Int).SetBytes(x.GetFee()).String(),
		Values:       values,
		N:            x.N,
		Extra:        hex.EncodeToString(x.GetExtra()),
	})
}

func (x *TxHeader) UnmarshalJSON(b []byte) error {
	var header jsonTxHeader
	err := utils.JSON.Unmarshal(b, &header)
	if err != nil {
		return err
	}

	x.Type = TxType(header.Type)
	x.Convener = uint64(header.Convener)

	x.Participants = make([][]byte, len(header.Participants))
	for i := range header.Participants {
		raw, err := base58.FastBase58Decoding(header.Participants[i])
		if err != nil {
			x.Participants = nil
			return err
		}
		x.Participants[i] = raw
	}

	bigFee, ok := new(big.Int).SetString(header.Fee, 10)
	if !ok {
		return fmt.Errorf("failed to parse txHeader's fee")
	}
	x.Fee = bigFee.Bytes()

	x.Values = make([][]byte, len(header.Values))
	for i := range header.Values {
		bigV, ok := new(big.Int).SetString(header.Values[i], 10)
		if !ok {
			x.Values = nil
			return fmt.Errorf("failed to parse txHeader's values")
		}
		x.Values[i] = bigV.Bytes()
	}

	x.N = header.N

	return nil
}

type jsonTx struct {
	Network int       `json:"network"`
	Header  *TxHeader `json:"header"`
	Sign    string    `json:"sign"`
}

func (x Tx) MarshalJSON() ([]byte, error) {
	mapTx := jsonTx{
		Network: int(x.GetNetwork()),
		Header:  x.GetHeader(),
		Sign:    hex.EncodeToString(x.GetSign()),
	}

	return utils.JSON.Marshal(&mapTx)
}

func (x *Tx) UnmarshalJSON(b []byte) error {
	var tx jsonTx
	err := utils.JSON.Unmarshal(b, &tx)
	if err != nil {
		return err
	}

	x.Network = NetworkType(tx.Network)
	x.Header = tx.Header

	x.Sign, err = hex.DecodeString(tx.Sign)
	if err != nil {
		return err
	}

	return nil
}
