package ngtypes

var genesisSheet *Sheet

// genesisBalances is empty: the chain starts with no premine. The old
// sheet carried balances under pre-quantum raw-key addresses; those
// were dropped when the secp scheme was removed
var genesisBalances = []*Balance{}

// GetGenesisSheet returns a genesis sheet: no balances, no contract
// slots — everything on this chain is earned and deployed after birth
func GetGenesisSheet(network Network) *Sheet {
	if genesisSheet == nil {
		genesisSheet = NewSheet(network, 0, GetGenesisBlock(network).GetHash(), genesisBalances, []*Contract{})
	}

	return genesisSheet
}
