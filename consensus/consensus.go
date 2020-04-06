package consensus

import (
	"crypto/ecdsa"
	"sync"

	"github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/txpool"
)

// Consensus is a prtoof on work consensus manager
type Consensus struct {
	sync.RWMutex

	isMining     bool
	SheetManager *ngsheet.Manager

	PrivateKey *ecdsa.PrivateKey
	Chain      *ngchain.Chain
	TxPool     *txpool.TxPool
	miner      *miner.Miner
}

// NewConsensusManager creates a new proof of work consensus manager
func NewConsensusManager(mining bool) *Consensus {
	return &Consensus{
		SheetManager: nil,
		PrivateKey:   nil,
		Chain:        nil,
		TxPool:       nil,

		isMining: mining,
	}
}

// Init will assemble the submodules into consensus
func (c *Consensus) Init(chain *ngchain.Chain, sheetManager *ngsheet.Manager, privateKey *ecdsa.PrivateKey, txPool *txpool.TxPool) {
	c.PrivateKey = privateKey
	c.SheetManager = sheetManager
	c.Chain = chain
	c.TxPool = txPool
}
