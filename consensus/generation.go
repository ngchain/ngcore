package consensus

import (
	"math/big"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// createGenerateTx will create a generate Tx for new Block.
func (pow *PoWork) createGenerateTx(privateKey *secp256k1.PrivateKey, extraData []byte) *ngtypes.Tx {
	publicKeyBytes := utils.PublicKey2Bytes(*privateKey.PubKey())
	gen := ngtypes.NewUnsignedTx(
		ngtypes.TxType_GENERATE,
		0,
		[][]byte{publicKeyBytes},
		[]*big.Int{ngtypes.OneBlockBigReward},
		ngtypes.GetBig0(),
		pow.state.GetNextNonce(0),
		extraData,
	)

	err := gen.Signature(privateKey)
	if err != nil {
		log.Error(err)
	}

	return gen
}
