package ngtypes

import (
	"encoding/hex"

	"github.com/ngchain/ngcore/utils"
)

type jsonAccount struct {
	Num   uint64  `json:"num"`
	Owner Address `json:"owner"`

	Contract string          `json:"contract"`
	Context  *AccountContext `json:"context"`
}

func (x *Account) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonAccount{
		Num:   x.Num,
		Owner: x.Owner,

		Contract: hex.EncodeToString(x.Contract),
		Context:  x.Context,
	})
}

func (x *Account) UnmarshalJSON(data []byte) error {
	var account jsonAccount
	err := utils.JSON.Unmarshal(data, &account)
	if err != nil {
		return err
	}

	contract, err := hex.DecodeString(account.Contract)
	if err != nil {
		return err
	}

	*x = *NewAccount(
		AccountNum(account.Num),
		account.Owner,
		contract,
		account.Context,
	)

	return nil
}
