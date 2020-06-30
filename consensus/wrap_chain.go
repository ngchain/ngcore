package consensus

import (
	"sync"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var chainMu sync.Mutex

// ApplyBlock checks the block and then calls ngchain's PutNewBlock, after which update the state
func (pow *PoWork) ApplyBlock(block *ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := pow.checkBlock(block); err != nil {
		return err
	}

	err := storage.GetChain().PutNewBlock(block)
	if err != nil {
		return err
	}

	ngstate.GetStateManager().UpdateState(block) // handle Block Txs inside

	return nil
}
