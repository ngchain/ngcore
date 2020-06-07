package ngtypes

import (
	"golang.org/x/crypto/sha3"

	"github.com/ngchain/ngcore/utils"
)

// NewSheet gets the rows from db and return the sheet for transport/saving.
func NewSheet(height uint64, accounts map[uint64]*Account, anonymous map[string][]byte) *Sheet {
	return &Sheet{
		Height:    height,
		Accounts:  accounts,
		Anonymous: anonymous,
	}
}

// CalculateHash mainly for calculating the tire root of txs and sign tx.
func (x *Sheet) Hash() []byte {
	raw, err := utils.Proto.Marshal(x)
	if err != nil {
		return nil
	}

	hash := sha3.Sum256(raw)

	return hash[:]
}

var GenesisSheet *Sheet

// GetGenesisSheetHash returns a genesis sheet's hash
func init() {
	accounts := make(map[uint64]*Account)

	for i := uint64(0); i <= 100; i++ {
		accounts[i] = GetGenesisStyleAccount(i)
	}

	GenesisSheet = &Sheet{
		Height:   0,
		Accounts: accounts,
		Anonymous: map[string][]byte{
			GenesisPublicKeyBase58: GetBig0Bytes(),
		},
	}
}
