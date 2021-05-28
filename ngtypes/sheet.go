package ngtypes

import (
	"github.com/ngchain/ngcore/ngtypes/ngproto"
)

type Sheet struct {
	ngproto.Sheet
}

// NewSheet gets the rows from db and return the sheet for transport/saving.
func NewSheet(network ngproto.NetworkType, height uint64, blockHash []byte, accounts map[uint64]*ngproto.Account, anonymous map[string][]byte) *Sheet {
	return &Sheet{
		ngproto.Sheet{
			Network:   network,
			Height:    height,
			BlockHash: blockHash,
			Anonymous: anonymous,
			Accounts:  accounts,
		},
	}
}

// GetGenesisSheet returns a genesis sheet
func GetGenesisSheet(network ngproto.NetworkType) *Sheet {
	accounts := make(map[uint64]*ngproto.Account)

	for i := uint64(0); i <= 100; i++ {
		accounts[i] = GetGenesisStyleAccount(AccountNum(i)).GetProto()
	}

	return NewSheet(network, 0, GetGenesisBlockHash(network), accounts, GenesisBalances)
}
