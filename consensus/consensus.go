package consensus

import (
	"crypto/ecdsa"
	"github.com/ngin-network/ngcore/chain"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/ngin-network/ngcore/sheetManager"
	"github.com/ngin-network/ngcore/txpool"
)

// the pow
type Consensus struct {
	template     *ngtypes.Block
	SheetManager *sheetManager.SheetManager

	privateKey *ecdsa.PrivateKey
	Chain      *chain.Chain

	TxPool *txpool.TxPool
}

func NewConsensusManager() *Consensus {
	return &Consensus{
		template:     nil,
		SheetManager: nil,
		privateKey:   nil,
		Chain:        nil,
		TxPool:       nil,
	}
}

func (c *Consensus) Init(chain *chain.Chain, sheetManager *sheetManager.SheetManager, privateKey *ecdsa.PrivateKey, txPool *txpool.TxPool) {
	c.privateKey = privateKey
	c.SheetManager = sheetManager
	c.Chain = chain
	c.TxPool = txPool
}
