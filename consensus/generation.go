package consensus

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
)

// CreateGeneration will create a generation Tx for new Block
func (c *Consensus) CreateGeneration(privateKey *ecdsa.PrivateKey, blockHeight uint64, extraData []byte) *ngtypes.Transaction {
	publicKeyBytes := elliptic.Marshal(privateKey.PublicKey, privateKey.PublicKey.X, privateKey.PublicKey.Y)
	gen := ngtypes.NewUnsignedTransaction(
		0,
		0,
		[][]byte{publicKeyBytes},
		[]*big.Int{ngtypes.OneBlockReward},
		ngtypes.Big0,
		blockHeight,
		extraData)
	err := gen.Signature(privateKey)
	if err != nil {
		log.Error(err)
	}

	return gen
}
