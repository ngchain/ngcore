package consensus

import (
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
)

// CreateGenerateTx will create a generate Tx for new Block.
// generate Tx is disallowed to edit external so use more local var.
func CreateGenerateTx(network ngtypes.Network, privateKey *ngtypes.PrivateKey, height uint64, extraData []byte) *ngtypes.FullTx {
	addr := ngtypes.NewAddress(privateKey)
	fee := big.NewInt(0)
	gen := ngtypes.NewUnsignedTx(
		network,
		ngtypes.GenerateTx,
		height,
		addr,
		ngtypes.GetBlockReward(height),
		fee,
		extraData,
	)

	err := gen.Signature(privateKey)
	if err != nil {
		log.Error(err)
	}

	return gen
}

// buildUncleRewardTxs returns one UNSIGNED generate per uncle, paying the
// uncle's coinbase the depth-decayed reward. They carry no signature: the
// state layer binds recipient+amount to the block's uncle set, so no signer
// is needed. Must be included (with the miner's own generate) before sealing.
func buildUncleRewardTxs(network ngtypes.Network, uncles []*ngtypes.BlockHeader, nephewHeight uint64) []*ngtypes.FullTx {
	txs := make([]*ngtypes.FullTx, 0, len(uncles))
	for _, u := range uncles {
		var to ngtypes.Address
		copy(to[:], u.Coinbase)
		txs = append(txs, ngtypes.NewUnsignedTx(
			network,
			ngtypes.GenerateTx,
			nephewHeight,
			to,
			ngtypes.UncleReward(u.Height, nephewHeight),
			big.NewInt(0),
			nil,
		))
	}
	return txs
}
