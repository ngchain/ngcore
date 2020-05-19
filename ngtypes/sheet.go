package ngtypes

import (
	"golang.org/x/crypto/sha3"

	"github.com/ngchain/ngcore/utils"
)

// NewSheet gets the rows from db and return the sheet for transport/saving.
func NewSheet(accounts map[uint64]*Account, anonymous map[string][]byte) *Sheet {
	return &Sheet{
		Accounts:  accounts,
		Anonymous: anonymous,
	}
}

// CalculateHash mainly for calculating the tire root of txs and sign tx.
func (x *Sheet) CalculateHash() ([]byte, error) {
	raw, err := utils.Proto.Marshal(x)
	if err != nil {
		return nil, err
	}

	hash := sha3.Sum256(raw)

	return hash[:], nil
}

// GetGenesisSheet returns a genesis sheet
func GetGenesisSheet() *Sheet {
	// reserve 1-100 to provide official functions
	accounts := make(map[uint64]*Account)

	for i := uint64(0); i <= 100; i++ {
		accounts[i] = GetGenesisStyleAccount(i)
	}

	return &Sheet{
		Accounts: accounts,
		Anonymous: map[string][]byte{
			GenesisPublicKeyBase58: GetBig0Bytes(),
		},
	}
}

var genesisSheetHash []byte

// GetGenesisSheetHash returns a genesis sheet's hash
func GetGenesisSheetHash() []byte {
	if genesisBlockHash == nil {
		genesisSheetHash, _ = GetGenesisSheet().CalculateHash()
	}

	return genesisSheetHash
}
