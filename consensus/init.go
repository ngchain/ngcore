package consensus

import (
	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/txpool"
)

// Init will assemble the submodules into consensus
func (c *Consensus) Init(chain *ngchain.Chain, sheetMgr *ngsheet.SheetManager, privateKey *secp256k1.PrivateKey, txPool *txpool.TxPool) {
	c.PrivateKey = privateKey
	c.SheetManager = sheetMgr
	c.Chain = chain
	c.TxPool = txPool
}
