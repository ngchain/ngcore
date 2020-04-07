package consensus

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// CreateGenerateTx will create a generate Tx for new Block
func (c *Consensus) CreateGenerateTx(privateKey *ecdsa.PrivateKey, blockHeight uint64, extraData []byte) *ngtypes.Tx {
	publicKeyBytes := utils.ECDSAPublicKey2Bytes(privateKey.PublicKey)
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
