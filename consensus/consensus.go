package consensus

import (
	"crypto/ecdsa"
	"github.com/ngchain/ngcore/chain"
	miner2 "github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/sheet"
	"github.com/ngchain/ngcore/txpool"
	"sync"
)

// the pow
type Consensus struct {
	sync.RWMutex

	template     *ngtypes.Block
	SheetManager *sheet.Manager

	privateKey *ecdsa.PrivateKey
	Chain      *chain.Chain

	TxPool *txpool.TxPool

	mining bool
	miner  *miner2.Miner
}

func NewConsensusManager(mining bool) *Consensus {
	return &Consensus{
		template:     nil,
		SheetManager: nil,
		privateKey:   nil,
		Chain:        nil,
		TxPool:       nil,

		mining: mining,
	}
}

func (c *Consensus) Init(chain *chain.Chain, sheetManager *sheet.Manager, privateKey *ecdsa.PrivateKey, txPool *txpool.TxPool) {
	c.privateKey = privateKey
	c.SheetManager = sheetManager
	c.Chain = chain
	c.TxPool = txPool
}
