package consensus

import (
	"sync"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/consensus/miner"
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

	isMining bool

	PrivateKey *secp256k1.PrivateKey
	miner      *miner.Miner
}

var consensus *PoWork

func NewConsensus(isMining bool, chain *ngchain.Chain, sheetMgr *ngsheet.StatusManager,
	privateKey *secp256k1.PrivateKey, txpool *txpool.TxPool, localNode *ngp2p.LocalNode) *PoWork {
	consensus = &PoWork{
		sheetManager: sheetMgr,
		chain:        chain,
		txpool:       txpool,
		localNode:    localNode,
		isMining:     isMining,
		PrivateKey:   privateKey,
		miner:        nil,
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
