package ngtypes

type Sheet struct {
	Network   Network
	Height    uint64
	BlockHash []byte
	Balances  []*Balance
	Contracts []*Contract
}

// NewSheet gets the rows from db and return the sheet for transport/saving.
func NewSheet(network Network, height uint64, blockHash []byte, balances []*Balance, contracts []*Contract) *Sheet {
	return &Sheet{
		Network:   network,
		Height:    height,
		BlockHash: blockHash,
		Balances:  balances,
		Contracts: contracts,
	}
}
