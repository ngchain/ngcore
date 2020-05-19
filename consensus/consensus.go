package consensus

import (
	"sync"

	"github.com/ngchain/secp256k1"
)

// PoWork is a proof on work consensus manager
type PoWork struct {
	sync.RWMutex

	syncMod  *syncModule
	minerMod *minerModule

	PrivateKey *secp256k1.PrivateKey
}

var pow *PoWork

// NewPoWConsensus creates and initializes the PoW consensus.
func NewPoWConsensus(miningThread int, privateKey *secp256k1.PrivateKey, isBootstrapNode bool) *PoWork {
	pow = &PoWork{
		RWMutex:    sync.RWMutex{},
		PrivateKey: privateKey,

		syncMod:  nil,
		minerMod: nil,
	}

	pow.minerMod = newMinerModule(pow, miningThread)
	pow.syncMod = newSyncModule(pow, isBootstrapNode)

	return pow
}

// GetPoWConsensus creates a new proof of work consensus manager.
func GetPoWConsensus() *PoWork {
	if pow == nil {
		panic("pow has not initialized")
	}

	return pow
}

// GoLoop ignites all loops
func (pow *PoWork) GoLoop() {
	go pow.loop()
	go pow.syncMod.loop()
}
