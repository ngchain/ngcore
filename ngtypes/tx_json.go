package ngtypes

import (
	"encoding/hex"
	"math/big"

	"github.com/ngchain/ngcore/utils"
)

type jsonTx struct {
	Network       int        `json:"network"`
	Type          int        `json:"type"`
	PrevBlockHash string     `json:"prev_block_hash"`
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
	participants := make([]Address, len(x.Participants))
	for i := range x.Participants {
		participants[i] = Address(x.Participants[i])
	}

	values := make([](*big.Int), len(x.Values))
	for i := range x.Values {
		values[i] = new(big.Int).SetBytes(x.Values[i])
	}

	return utils.JSON.Marshal(jsonTx{
		Network:       int(x.Network),
		Type:          int(x.GetType()),
		PrevBlockHash: hex.EncodeToString(x.PrevBlockHash),
		Convener:      x.Convener,
		Participants:  participants,
		Fee:           new(big.Int).SetBytes(x.GetFee()),
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

	network := NetworkType(tx.Network)

	t := TxType(tx.Type)

	prevBlockHash, err := hex.DecodeString(tx.PrevBlockHash)
	if err != nil {
		return err
	}

	convener := uint64(tx.Convener)

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

	*x = Tx{
		Network:       network,
		Type:          t,
		PrevBlockHash: prevBlockHash,
		Convener:      convener,
		Participants:  participants,
		Fee:           tx.Fee.Bytes(),
		Values:        values,
		Extra:         extra,
		Sign:          sign,
	}

	return nil
}
