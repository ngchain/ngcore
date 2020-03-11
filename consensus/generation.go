package consensus

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"github.com/ngin-network/ngcore/ngtypes"
	"math/big"
)

func (c *Consensus) CreateGeneration(privateKey *ecdsa.PrivateKey, blockHeight uint64, currentBlockHash, currentVaultHash, extraData []byte) *ngtypes.Transaction {
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
