package ngtypes

import (
	"encoding/hex"
	"math/big"

	"github.com/ngchain/ngcore/utils"
)

type jsonTx struct {
	Network      uint8      `json:"network"`
	Type         uint8      `json:"type"`
	Height       uint64     `json:"prevBlockHash"`
	Convener     AccountNum `json:"convener"`
	Participants []Address  `json:"participants"`
	Fee          *big.Int   `json:"fee"`
	Values       []*big.Int `json:"values"`
	Extra        string     `json:"extra"`

	Sign string `json:"sign"`

	// helpers
	Hash string `json:"hash,omitempty"`
}

func (x *Tx) MarshalJSON() ([]byte, error) {

	return utils.JSON.Marshal(jsonTx{
		Network:      x.Network,
		Type:         x.Type,
		Height:       x.Height,
		Convener:     x.Convener,
		Participants: x.Participants,
		Fee:          x.Fee,
		Values:       x.Values,
		Extra:        hex.EncodeToString(x.Extra),

		Sign: hex.EncodeToString(x.Sign),

		Hash: hex.EncodeToString(x.GetHash()),
	})
}

func (x *Tx) UnmarshalJSON(b []byte) error {
	var tx jsonTx
	err := utils.JSON.Unmarshal(b, &tx)
	if err != nil {
		return err
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

	*x = *NewTx(
		tx.Network,
		tx.Type,
		tx.Height,
		tx.Convener,
		tx.Participants,
		tx.Values,
		tx.Fee,
		extra,
		sign,
		hash,
	)

	return nil
}
