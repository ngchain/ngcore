package ngtypes

import (
	"encoding/hex"

	"github.com/mr-tron/base58"
	"github.com/ngchain/ngcore/utils"
)

// NewAccount receive parameters and return a new Account(class constructor.
func NewAccount(num uint64, ownerPublicKey []byte, contract, context []byte) *Account {
	return &Account{
		Num:      num,
		Owner:    ownerPublicKey,
		Contract: contract,
		Context:  context,
	}
}

// GetGenesisStyleAccount will return the genesis style account.
func GetGenesisStyleAccount(num uint64) *Account {
	return &Account{
		Num:      num,
		Owner:    GenesisAddress,
		Contract: nil,
		Context:  nil,
	}
}

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

	x.Num = account.Num
	x.Owner, err = base58.FastBase58Decoding(account.Owner)
	if err != nil {
		return err
	}

	x.Contract, err = hex.DecodeString(account.Contract)
	if err != nil {
		return err
	}

	x.Context, err = hex.DecodeString(account.Context)
	if err != nil {
		return err
	}

	return nil
}
