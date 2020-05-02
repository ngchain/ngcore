package consensus

import (
	"sync"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/txpool"
)

// PoWork is a proof on work consensus manager
type PoWork struct {
	sync.RWMutex

	sheetManager *ngsheet.StatusManager
	chain        *ngchain.Chain
	txpool       *txpool.TxPool
	localNode    *ngp2p.LocalNode

	// duck
	*syncModule
	*minerModule

	isMining bool

	PrivateKey  *secp256k1.PrivateKey
}

var consensus *PoWork

// NewConsensus creates and initizlizes the PoW consensus
func NewConsensus(isMining bool, chain *ngchain.Chain, sheetMgr *ngsheet.StatusManager,
	privateKey *secp256k1.PrivateKey, txpool *txpool.TxPool, localNode *ngp2p.LocalNode) *PoWork {
	consensus = &PoWork{
		RWMutex:      sync.RWMutex{},
		sheetManager: sheetMgr,
		chain:        chain,
		txpool:       txpool,
		localNode:    localNode,
		syncModule:   &syncModule{},
		isMining:     isMining,
		PrivateKey:   privateKey,
		minerModule:  nil,
	}

	return consensus
}

// GetConsensus creates a new proof of work consensus manager.
func GetConsensus() *PoWork {
	if consensus == nil {
		consensus = &PoWork{}
	}

	return consensus
}
