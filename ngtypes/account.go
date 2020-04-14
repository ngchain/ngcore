package ngtypes

import (
	"github.com/ngchain/ngcore/utils"
)

// NewAccount receive parameters and return a new Account(class constructor
func NewAccount(num uint64, ownerPublicKey []byte, contract, state []byte) *Account {
	return &Account{
		Num:      num,
		Owner:    ownerPublicKey,
		Nonce:    0,
		Contract: contract,
		State:    state,
	}
}

// GetGenesisAccount will return the genesis account (id=1)
func GetGenesisAccount(num uint64) *Account {
	return &Account{
		Num: num,
		// Balance:  big.NewInt(math.MaxInt64).Bytes(), // Init balance
		Owner: GenesisPublicKey,
		Nonce: 0,
		State: genesisState,
	}
}

var genesisState, _ = utils.JSON.Marshal(map[string]interface{}{
	"name": "ngchain",
})
