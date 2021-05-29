package ngtypes

import (
	"math/big"

	"github.com/ngchain/ngcore/ngtypes/ngproto"
)

var genesisGenerateTx *Tx

func GetGenesisGenerateTx(network ngproto.NetworkType) *Tx {
	if genesisGenerateTx == nil {
		ggtx := NewTx(network, ngproto.TxType_GENERATE, nil, 0, [][]byte{GenesisAddress},
			BigIntsToBytesList([]*big.Int{GetBlockReward(0)}),
			big.NewInt(0).Bytes(),
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
