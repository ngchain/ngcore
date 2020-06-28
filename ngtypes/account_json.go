package ngtypes

import (
	"encoding/hex"

	"github.com/mr-tron/base58"
	"github.com/ngchain/ngcore/utils"
)

type jsonAccount struct {
	Num   uint64 `json:"num"`
	Owner string `json:"owner"`

	Contract string `json:"contract"`
	Context  string `json:"state"`
}

func (x *Account) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonAccount{
		Num:   x.GetNum(),
		Owner: base58.FastBase58Encoding(x.GetOwner()),

		Contract: hex.EncodeToString(x.GetContract()),
		Context:  hex.EncodeToString(x.GetContext()),
	})
}

func (x *Account) UnmarshalJSON(data []byte) error {
	var account jsonAccount
	err := utils.JSON.Unmarshal(data, &account)
	if err != nil {
		return err
	}

	owner, err := base58.FastBase58Decoding(account.Owner)
	if err != nil {
		return err
	}

	contract, err := hex.DecodeString(account.Contract)
	if err != nil {
		return err
	}

	context, err := hex.DecodeString(account.Context)
	if err != nil {
		return err
	}

	*x = Account{
		Num:      account.Num,
		Owner:    owner,
		Contract: contract,
		Context:  context,
	}

	return nil
}
