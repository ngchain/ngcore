package ngtypes

import "github.com/ngchain/ngcore/ngtypes/ngproto"

var genesisBlock *Block

// GetGenesisBlock will return a complete sealed GenesisBlock.
func GetGenesisBlock(network ngproto.NetworkType) *Block {
	txs := []*Tx{
		GetGenesisGenerateTx(network),
	}

	if genesisBlock == nil {
		genesisBlock = NewBlock(
			network,
			0,
			GetGenesisTimestamp(network),

			make([]byte, HashSize),
			NewTxTrie(txs).TrieRoot(),

			minimumBigDifficulty.Bytes(), // this is a number, dont put any padding on
			GetGenesisBlockNonce(network),
			[]*ngproto.BlockHeader{},
			txs,
			nil,
		)
		genesisBlock.GetHash()
	}

	return genesisBlock
}

func GetGenesisBlockHash(network ngproto.NetworkType) []byte {
	return GetGenesisBlock(network).GetHash()
}
