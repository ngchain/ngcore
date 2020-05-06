package consensus

import (
	"sync"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/txpool"
)

// PoWork is a proof on work consensus manager
type PoWork struct {
	sync.RWMutex

	sheetManager *ngsheet.StatusManager
	chain        *storage.Chain
	txpool       *txpool.TxPool
	localNode    *ngp2p.LocalNode

	// duck
	*syncModule
	*minerModule

	isMining bool

	PrivateKey *secp256k1.PrivateKey
}

var pow *PoWork

// NewConsensus creates and initializes the PoW consensus
func NewConsensus(isMining bool, chain *storage.Chain, sheetMgr *ngsheet.StatusManager,
	privateKey *secp256k1.PrivateKey, txpool *txpool.TxPool, localNode *ngp2p.LocalNode) *PoWork {
	pow = &PoWork{
		RWMutex:      sync.RWMutex{},
		sheetManager: sheetMgr,
		chain:        chain,
		txpool:       txpool,
		localNode:    localNode,
		syncModule:   nil,
		isMining:     isMining,
		PrivateKey:   privateKey,
		minerModule:  nil,
	}

	pow.syncModule = newSyncModule(pow)

	return pow
}

// GetConsensus creates a new proof of work consensus manager.
func GetConsensus() *PoWork {
	if pow == nil {
		pow = &PoWork{}
	}

	return pow
}
