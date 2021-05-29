package ngtypes

import (
	"encoding/hex"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes/ngproto"
	"github.com/ngchain/ngcore/utils"
)

type jsonTx struct {
	Network       int        `json:"network"`
	Type          int        `json:"type"`
	PrevBlockHash string     `json:"prevBlockHash"`
	Convener      uint64     `json:"convener"`
	Participants  []Address  `json:"participants"`
	Fee           *big.Int   `json:"fee"`
	Values        []*big.Int `json:"values"`
	Extra         string     `json:"extra"`

	Sign string `json:"sign"`

	// helpers
	Hash string `json:"hash,omitempty"`
}

func (x *Tx) MarshalJSON() ([]byte, error) {
	participants := make([]Address, len(x.Proto.Participants))
	for i := range x.Proto.Participants {
		participants[i] = x.Proto.Participants[i]
	}

	values := make([]*big.Int, len(x.Proto.Values))
	for i := range x.Proto.Values {
		values[i] = new(big.Int).SetBytes(x.Proto.Values[i])
	}

	return utils.JSON.Marshal(jsonTx{
		Network:       int(x.Proto.Network),
		Type:          int(x.Proto.GetType()),
		PrevBlockHash: hex.EncodeToString(x.Proto.PrevBlockHash),
		Convener:      x.Proto.Convener,
		Participants:  participants,
		Fee:           new(big.Int).SetBytes(x.Proto.GetFee()),
		Values:        values,
		Extra:         hex.EncodeToString(x.Proto.GetExtra()),

		Sign: hex.EncodeToString(x.Proto.GetSign()),

		Hash: hex.EncodeToString(x.GetHash()),
	})
}

func (x *Tx) UnmarshalJSON(b []byte) error {
	var tx jsonTx
	err := utils.JSON.Unmarshal(b, &tx)
	if err != nil {
		return err
	}

	network := ngproto.NetworkType(tx.Network)

	t := ngproto.TxType(tx.Type)

	prevBlockHash, err := hex.DecodeString(tx.PrevBlockHash)
	if err != nil {
		return err
	}

	convener := tx.Convener

	participants := make([][]byte, len(tx.Participants))
	for i := range tx.Participants {
		participants[i] = tx.Participants[i]
	}

	values := make([][]byte, len(tx.Values))
	for i := range tx.Values {
		values[i] = tx.Values[i].Bytes()
	}

	extra, err := hex.DecodeString(tx.Extra)
	if err != nil {
		return err
	}

	sign, err := hex.DecodeString(tx.Sign)
	if err != nil {
		return err
	}

	hash, err := hex.DecodeString(tx.Hash)
	if err != nil {
		return err
	}

	*x = *NewTx(network,
		t,
		prevBlockHash,
		convener,
		participants,
		values,
		tx.Fee.Bytes(),
		extra,
		sign,
		hash,
	)

	return nil
}
