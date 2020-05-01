package ngtypes

import (
	"github.com/ngchain/ngcore/utils"
)

// NewAccount receive parameters and return a new Account(class constructor.
func NewAccount(num uint64, ownerPublicKey []byte, contract, state []byte) *Account {
	return &Account{
		Num:      num,
		Owner:    ownerPublicKey,
		Txn:      0,
		Contract: contract,
		State:    state,
	}
}

// GetGenesisStyleAccount will return the genesis style account.
func GetGenesisStyleAccount(num uint64) *Account {
	return &Account{
		Num: num,
		// Balance:  big.NewInt(math.MaxInt64).Bytes(), // Init balance
		Owner: GenesisPublicKey,
		Txn:   0,
		State: genesisState,
	}
}

var genesisState, _ = utils.JSON.Marshal(map[string]interface{}{
	"name": "ngchain",
})
