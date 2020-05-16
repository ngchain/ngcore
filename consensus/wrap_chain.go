package consensus

import (
	"fmt"
	"sync"

	"github.com/ngchain/ngcore/ngtypes"
)

var chainMu sync.Mutex

func (pow *PoWork) initWithChain(chain ...*ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := pow.checkChain(chain...); err != nil {
		return fmt.Errorf("chain invalid: %s", err)
	}

	return pow.chain.InitWithChain(chain...)
}

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
