package ngtypes

// AccountID will receive an ID and PK then return a Account without SubState and Balance(0

// NewAccount receive parameters and return a new Account(class constructor
func NewAccount(id uint64, ownerKey []byte, state []byte) *Account {
	return &Account{
		ID:    id,
		Owner: ownerKey,
		State: state,
	}
}

//func NewRewardAccount(id uint64, ownerKey []byte, totalFeeReward *big.Int) *Account {
//	reward := new(big.Int).Add(OneBlockReward, totalFeeReward)
//	return NewAccount(id, ownerKey, reward, nil)
//}

// GetGenesisAccount is a default class constructor
func GetGenesisAccount() *Account {
	return &Account{
		ID: 1,
		//Balance:  big.NewInt(math.MaxInt64).Bytes(), // Init balance
		Owner: GenesisPK,
		Nonce: 0,
		State: []byte(`{'name':'NGIN OFFICIAL'}`),
	}
}
