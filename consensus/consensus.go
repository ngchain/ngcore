package consensus

import (
	"sync"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/txpool"
)

// PoWork is a proof on work consensus manager
type PoWork struct {
	sync.RWMutex

	state     *ngstate.State
	chain     *storage.Chain
	txpool    *txpool.TxPool
	localNode *ngp2p.LocalNode

	syncMod  *syncModule
	minerMod *minerModule

	PrivateKey *secp256k1.PrivateKey
}

var pow *PoWork

// NewPoWConsensus creates and initializes the PoW consensus
func NewPoWConsensus(miningThread int, chain *storage.Chain, privateKey *secp256k1.PrivateKey, localNode *ngp2p.LocalNode) *PoWork {
	state, _ := ngstate.NewStateFromSheet(ngtypes.GetGenesisSheet())
	txpool := txpool.NewTxPool(state)

	pow = &PoWork{
		RWMutex:    sync.RWMutex{},
		state:      state,
		chain:      chain,
		txpool:     txpool,
		localNode:  localNode,
		PrivateKey: privateKey,

		syncMod:  nil,
		minerMod: nil,
	}

	pow.minerMod = newMinerModule(miningThread)
	pow.syncMod = newSyncModule(pow)

	return pow
}

// GetPoWConsensus creates a new proof of work consensus manager.
func GetPoWConsensus() *PoWork {
	if pow == nil {
		panic("pow has not initialized")
	}

	return pow
}

func (c *PoWork) Loop() {
	c.syncMod.loop()

	if pow.minerMod != nil {
		pow.minerMod.start(pow.getBlockTemplate())

		go func() {
			for {
				b := <-pow.minerMod.FoundBlockCh
				pow.minedNewBlock(b)
				// assign new job
				pow.minerMod.start(pow.getBlockTemplate())
			}
		}()
	}
}

func (c *PoWork) initTxPool(state *ngstate.State) {
	txpool := txpool.NewTxPool(state)
	txpool.Init(c.chain.MinedBlockToTxPoolCh)
	txpool.Run()
}
