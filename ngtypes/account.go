package ngtypes

// NewAccount receive parameters and return a new Account(class constructor.
func NewAccount(num uint64, ownerPublicKey []byte, contract, context []byte) *Account {
	return &Account{
		Num:      num,
		Owner:    ownerPublicKey,
		Contract: contract,
		Context:  context,
	}
}

// GetGenesisStyleAccount will return the genesis style account.
func GetGenesisStyleAccount(num uint64) *Account {
	return &Account{
		Num:      num,
		Owner:    GenesisAddress,
		Contract: nil,
		Context:  nil,
	}
}
