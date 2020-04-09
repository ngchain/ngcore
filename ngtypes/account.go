package ngtypes

// NewAccount receive parameters and return a new Account(class constructor
func NewAccount(id uint64, ownerKey []byte, state []byte) *Account {
	return &Account{
		ID:    id,
		Owner: ownerKey,
		State: state,
	}
}

// GetGenesisAccount will return the genesis account (id=1)
func GetGenesisAccount() *Account {
	return &Account{
		ID: 1,
		// Balance:  big.NewInt(math.MaxInt64).Bytes(), // Init balance
		Owner: GenesisPublicKey,
		Nonce: 0,
		State: []byte(`{'name':'NGIN OFFICIAL'}`),
	}
}
