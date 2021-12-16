package ngtypes

var genesisBlock *FullBlock

// GetGenesisBlock will return a complete sealed GenesisBlock.
func GetGenesisBlock(network Network) *FullBlock {
	txs := []*FullTx{
		GetGenesisGenerateTx(network),
	}

	if genesisBlock == nil {
		txTrie := NewTxTrie(txs)
		headerTrie := NewHeaderTrie(nil)
		genesisBlock = NewBlock(
			network,
			0,
			GetGenesisTimestamp(network),

			make([]byte, HashSize),
			txTrie.TrieRoot(),
			headerTrie.TrieRoot(),
			minimumBigDifficulty.Bytes(), // this is a number, dont put any padding on
			GetGenesisBlockNonce(network),
			txs,
			[]*BlockHeader{},
		)
		genesisBlock.GetHash()
	}

	return genesisBlock
}
