package consensus

import (
	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/txpool"
)

// Init will assemble the submodules into consensus
func (c *Consensus) Init(chain *ngchain.Chain, sheet *ngsheet.Sheet, privateKey *secp256k1.PrivateKey, txPool *txpool.TxPool) {
	c.PrivateKey = privateKey
	c.Sheet = sheet
	c.Chain = chain
	c.TxPool = txPool
}
