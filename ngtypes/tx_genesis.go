package ngtypes

import (
	"math/big"
)

var genesisGenerateTx *Tx

func GetGenesisGenerateTx(network uint8) *Tx {
	if genesisGenerateTx == nil {
		ggtx := NewTx(network, GenerateTx, 0, 0, []Address{GenesisAddress},
			[]*big.Int{GetBlockReward(0)},
			big.NewInt(0),
			nil,
			nil,
			nil,
		)

		ggtx.ManuallySetSignature(
			GetGenesisGenerateTxSignature(network))
		ggtx.GetHash()

		genesisGenerateTx = ggtx
	}

	return genesisGenerateTx
}
