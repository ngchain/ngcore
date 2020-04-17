package consensus

import (
	"sync"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/consensus/miner"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/txpool"
)

// Consensus is a prtoof on work consensus manager
type Consensus struct {
	sync.RWMutex

	*ngsheet.SheetManager
	*ngchain.Chain
	*txpool.TxPool

	isMining bool

	PrivateKey *secp256k1.PrivateKey
	miner      *miner.Miner
}

var consensus *Consensus

// GetConsensus creates a new proof of work consensus manager
func GetConsensus() *Consensus {
	if consensus == nil {
		consensus = &Consensus{}
	}

	return consensus
}
