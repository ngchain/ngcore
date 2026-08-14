package ngtypes

import (
	"encoding/hex"

	"github.com/ngchain/ngcore/utils"
)

type jsonAccount struct {
	Owner Address `json:"owner"`

	Contract string          `json:"contract"`
	Context  *AccountContext `json:"context"`
}

// MarshalJSON converts the Account into json bytes
func (x *Account) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonAccount{
		Owner: x.Owner,

		Contract: hex.EncodeToString(x.Contract),
		Context:  x.Context,
	})
}

// UnmarshalJSON recovers the Account from the json bytes
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
		account.Owner,
		contract,
		account.Context,
	)

	return nil
}
