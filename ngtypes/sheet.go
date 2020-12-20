package ngtypes

// NewSheet gets the rows from db and return the sheet for transport/saving.
func NewSheet(prevBlockHash []byte, accounts map[uint64]*Account, anonymous map[string][]byte) *Sheet {
	return &Sheet{
		PrevBlockHash: prevBlockHash,
		Anonymous:     anonymous,
		Accounts:      accounts,
	}
}

var GenesisSheet *Sheet

// GetGenesisSheetHash returns a genesis sheet's hash
func init() {
	accounts := make(map[uint64]*Account)

	for i := uint64(0); i <= 100; i++ {
		accounts[i] = GetGenesisStyleAccount(AccountNum(i))
	}

	GenesisSheet = &Sheet{
		PrevBlockHash: make([]byte, HashSize),
		Accounts:      accounts,
		Anonymous:     GenesisBalances,
	}
}
