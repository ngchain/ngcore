package ngtypes

var genesisSheet *Sheet

// genesisBalances is empty: the chain starts with no premine. The old
// sheet carried balances under pre-quantum raw-key addresses; those
// were dropped when the secp scheme was removed
var genesisBalances = []*Balance{}

// GetGenesisSheet returns a genesis sheet
func GetGenesisSheet(network Network) *Sheet {
	if genesisSheet == nil {
		accounts := make([]*Account, 0, 100)

		for i := uint64(0); i <= 100; i++ {
			accounts = append(accounts, GetGenesisStyleAccount(AccountNum(i)))
		}

		genesisSheet = NewSheet(network, 0, GetGenesisBlock(network).GetHash(), genesisBalances, accounts)
	}

	return genesisSheet
}
