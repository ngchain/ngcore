package ngtypes

import (
	"math/big"
)

var genesisGenerateTx *Tx

// GetGenesisGenerateTx provides the genesis generate tx under current network
func GetGenesisGenerateTx(network Network) *Tx {
	if genesisGenerateTx == nil || genesisGenerateTx.Network != network {
		ggtx := NewTx(network, GenerateTx, 0, 0, []Address{GenesisAddress},
			[]*big.Int{GetBlockReward(0)},
			big.NewInt(0),
			nil,
			nil,
		)

		ggtx.ManuallySetSignature(GetGenesisGenerateTxSignature(network))

		genesisGenerateTx = ggtx
	}

	return genesisGenerateTx
}
