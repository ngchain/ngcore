package consensus

import (
	"math/big"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// createGenerateTx will create a generate Tx for new Block
func (c *Consensus) createGenerateTx(privateKey *secp256k1.PrivateKey, extraData []byte) *ngtypes.Tx {
	publicKeyBytes := utils.PublicKey2Bytes(*privateKey.PubKey())
	gen := ngtypes.NewUnsignedTx(
		ngtypes.TX_GENERATE,
		0,
		[][]byte{publicKeyBytes},
		[]*big.Int{ngtypes.OneBlockReward},
		ngtypes.GetBig0(),
		c.GetNextNonce(0),
		extraData)
	err := gen.Signature(privateKey)
	if err != nil {
		log.Error(err)
	}

	return gen
}
