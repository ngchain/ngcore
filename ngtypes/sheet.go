package ngtypes

type Sheet struct {
	Network   uint8
	Height    uint64
	BlockHash []byte
	Balances  []*Balance
	Accounts  []*Account
}

// NewSheet gets the rows from db and return the sheet for transport/saving.
func NewSheet(network uint8, height uint64, blockHash []byte, balances []*Balance, accounts []*Account) *Sheet {
	return &Sheet{
		Network:   network,
		Height:    height,
		BlockHash: blockHash,
		Balances:  balances,
		Accounts:  accounts,
	}
}
