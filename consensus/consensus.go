package consensus

import (
	"crypto/ecdsa"
	"github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/txpool"
	"sync"
)

// the pow
type Consensus struct {
	sync.RWMutex

	SheetManager *ngsheet.Manager

	PrivateKey *ecdsa.PrivateKey
	Chain      *ngchain.Chain

	TxPool *txpool.TxPool

	isMining bool
	miner    *miner.Miner
}

func NewConsensusManager(mining bool) *Consensus {
	return &Consensus{
		SheetManager: nil,
		PrivateKey:   nil,
		Chain:        nil,
		TxPool:       nil,

		isMining: mining,
	}
}

func (c *Consensus) Init(chain *ngchain.Chain, sheetManager *ngsheet.Manager, privateKey *ecdsa.PrivateKey, txPool *txpool.TxPool) {
	c.PrivateKey = privateKey
	c.SheetManager = sheetManager
	c.Chain = chain
	c.TxPool = txPool
}
