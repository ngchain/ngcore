package consensus

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// CreateGeneration will create a generation Tx for new Block
func (c *Consensus) CreateGeneration(privateKey *ecdsa.PrivateKey, blockHeight uint64, extraData []byte) *ngtypes.Transaction {
	publicKeyBytes := utils.ECDSAPublicKey2Bytes(privateKey.PublicKey)
	gen := ngtypes.NewUnsignedTransaction(
		ngtypes.TX_GENERATION,
		0,
		[][]byte{publicKeyBytes},
		[]*big.Int{ngtypes.OneBlockReward},
		ngtypes.GetBig0(),
		blockHeight+1,
		extraData)
	err := gen.Signature(privateKey)
	if err != nil {
		log.Error(err)
	}

	return gen
}
