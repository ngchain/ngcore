package ngtypes

import (
	"encoding/hex"
	"math/big"

	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/utils"
)

type jsonTx struct {
	Network string   `json:"network"`
	Type    TxType   `json:"type"`
	Height  uint64   `json:"height"`
	To      Address  `json:"to"`
	Value   *big.Int `json:"value"`
	Fee     *big.Int `json:"fee"`
	Extra   string   `json:"extra"`

	Sign string `json:"sign"`

	// helpers
	Hash string `json:"hash,omitempty"`
}

// MarshalJSON encodes the tx into the json bytes
func (x *FullTx) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonTx{
		Network: x.Network.String(),
		Type:    x.Type,
		Height:  x.Height,
		To:      x.To,
		Value:   x.Value,
		Fee:     x.Fee,
		Extra:   hex.EncodeToString(x.Extra),

		Sign: hex.EncodeToString(x.Sign),

		Hash: hex.EncodeToString(x.GetHash()),
	})
}

// UnmarshalJSON decodes the Tx from the json bytes
func (x *FullTx) UnmarshalJSON(b []byte) error {
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

	network, err := ParseNetwork(tx.Network)
	if err != nil {
		return err
	}

	// negative amounts are unrepresentable (RLP encodes unsigned); a
	// JSON-sourced negative Value/Fee would otherwise panic later on
	// GetHash/Signature/MarshalJSON — reject it here instead
	if (tx.Value != nil && tx.Value.Sign() < 0) || (tx.Fee != nil && tx.Fee.Sign() < 0) {
		return errors.New("tx value and fee must be non-negative")
	}

	*x = *NewTx(
		network,
		tx.Type,
		tx.Height,
		tx.To,
		tx.Value,
		tx.Fee,
		extra,
		sign,
	)

	return nil
}
