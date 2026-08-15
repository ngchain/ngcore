package ngtypes

type Sheet struct {
	Network   Network
	Height    uint64
	BlockHash []byte
	Balances  []*Balance
	Contracts []*Contract
	// Keys carries the on-chain key registry (address -> scheme ‖
	// pubkey): snapshot-synced nodes must resolve compact envelopes
	// exactly like replaying nodes do
	Keys []*RegisteredKey
}

// RegisteredKey is one key-registry row inside a sheet
type RegisteredKey struct {
	Address Address
	Entry   []byte // scheme ‖ pubkey
}

// NewSheet gets the rows from db and return the sheet for transport/saving.
func NewSheet(network Network, height uint64, blockHash []byte, balances []*Balance, contracts []*Contract, keys []*RegisteredKey) *Sheet {
	return &Sheet{
		Network:   network,
		Height:    height,
		BlockHash: blockHash,
		Balances:  balances,
		Contracts: contracts,
		Keys:      keys,
	}
}
