package ngtypes

// NewAccount receive parameters and return a new Account(class constructor
func NewAccount(id uint64, ownerPublicKey []byte, state []byte) *Account {
	return &Account{
		ID:    id,
		Owner: ownerPublicKey,
		State: state,
	}
}

// GetGenesisAccount will return the genesis account (id=1)
func GetGenesisAccount(id uint64) *Account {
	return &Account{
		ID: id,
		// Balance:  big.NewInt(math.MaxInt64).Bytes(), // Init balance
		Owner: GenesisPublicKey,
		Nonce: 0,
		State: []byte(`{'name':'NGIN OFFICIAL'}`),
	}
}
