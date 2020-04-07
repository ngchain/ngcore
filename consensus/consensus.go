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

	*ngsheet.Sheet
	*ngchain.Chain
	*txpool.TxPool

	isMining bool

	PrivateKey *ecdsa.PrivateKey
	miner      *miner.Miner
}

// NewConsensus creates a new proof of work consensus manager
func NewConsensus(mining bool) *Consensus {
	return &Consensus{
		Sheet:      nil,
		Chain:      nil,
		TxPool:     nil,
		isMining:   mining,
		PrivateKey: nil,
		miner:      nil,
	}
}
