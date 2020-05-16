package consensus

import (
	"sync"

	"github.com/ngchain/ngcore/ngtypes"
)

var chainMu sync.Mutex

// PutNewBlock calls ngchain's PutNewBlock
func (pow *PoWork) PutNewBlock(block *ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := pow.checkBlock(block); err != nil {
		return err
	}

	err := pow.chain.PutNewBlock(block)
	if err != nil {
		return err
	}

	pow.txpool.HandleNewBlock(block)
	err = pow.state.HandleTxs(block.Txs...)
	if err != nil {
		return err
	}

	return nil
}
