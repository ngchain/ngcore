package consensus

import (
	"sync"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var chainMu sync.Mutex

// PutNewBlock calls ngchain's PutNewBlock
func (pow *PoWork) PutNewBlock(block *ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := pow.checkBlock(block); err != nil {
		return err
	}

	err := storage.GetChain().PutNewBlock(block)
	if err != nil {
		return err
	}

	ngstate.GetStateManager().UpdateState(block)

	err = ngstate.GetCurrentState().HandleTxs(block.Txs...)
	if err != nil {
		return err
	}

	return nil
}
