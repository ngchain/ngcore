package consensus

import (
	"math/big"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngtypes"
)

// CreateGenerateTx will create a generate Tx for new Block.
// generate Tx is disallowed to edit external so use more local var.
func CreateGenerateTx(network ngtypes.Network, privateKey *secp256k1.PrivateKey, height uint64, extraData []byte) *ngtypes.FullTx {
	addr := ngtypes.NewAddress(privateKey)
	fee := big.NewInt(0)
	gen := ngtypes.NewUnsignedTx(
		network,
		ngtypes.GenerateTx,
		height,
		0,
		[]ngtypes.Address{addr},
		[]*big.Int{ngtypes.GetBlockReward(height)},
		fee,
		extraData,
	)

	err := gen.Signature(privateKey)
	if err != nil {
		log.Error(err)
	}

	return gen
}
