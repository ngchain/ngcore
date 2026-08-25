package ngtypes

var genesisBlock *FullBlock

// GetGenesisBlock will return a complete sealed GenesisBlock.
func GetGenesisBlock(network Network) *FullBlock {
	txs := []*FullTx{
		GetGenesisGenerateTx(network),
	}

	// memoize per network: caching on nil alone froze the singleton to the
	// FIRST caller's network, so GetGenesisBlock(TESTNET) after ZERONET
	// returned the ZERONET block (GetGenesisGenerateTx already re-checks
	// this way)
	if genesisBlock == nil || genesisBlock.Network != network {
		txTrie := NewTxTrie(txs)
		genesisBlock = NewBlock(
			network,
			0,
			GetGenesisTimestamp(network),

			make([]byte, HashSize),
			txTrie.TrieRoot(),
			CalcWitnessRoot(txs, nil),
			MinimumDiffOf(network).Bytes(), // this is a number, dont put any padding on
			GetGenesisBlockNonce(network),
			txs,
		)
		genesisBlock.SetCoinbase(GenesisAddress)
		genesisBlock.GetHash()
	}

	return genesisBlock
}
