package ngtypes

import (
	"encoding/hex"

	"github.com/ngchain/ngcore/utils"
)

type jsonContract struct {
	Owner Address `json:"owner"`

	Source  string           `json:"source"` // hex of the contract text
	Context *ContractContext `json:"context"`
}

// MarshalJSON converts the Contract into json bytes
func (x *Contract) MarshalJSON() ([]byte, error) {
	return utils.JSON.Marshal(jsonContract{
		Owner: x.Owner,

		Source:  hex.EncodeToString(x.Source),
		Context: x.Context,
	})
}

// UnmarshalJSON recovers the Contract from the json bytes
func (x *Contract) UnmarshalJSON(data []byte) error {
	var contract jsonContract
	err := utils.JSON.Unmarshal(data, &contract)
	if err != nil {
		return err
	}

	source, err := hex.DecodeString(contract.Source)
	if err != nil {
		return err
	}

	*x = *NewContract(
		contract.Owner,
		source,
		contract.Context,
	)

	return nil
}
