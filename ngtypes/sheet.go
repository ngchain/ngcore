package ngtypes

import (
	"github.com/ngchain/ngcore/utils"
	"golang.org/x/crypto/sha3"
)

// NewSheet gets the rows from db and return the sheet for transport/saving.
func NewSheet(prevBlockHash []byte, accounts map[uint64]*Account, anonymous map[string][]byte) *Sheet {
	return &Sheet{
		PrevBlockHash: prevBlockHash,
		Anonymous:     anonymous,
		Accounts:      accounts,
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
		PrevBlockHash: nil,
		Accounts:      accounts,
		Anonymous: map[string][]byte{
			GenesisAddress.String(): GetBig0Bytes(),
		},
	}
}
