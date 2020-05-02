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

// func (c *PoWork) ForkToNewChain(chain ...*ngtypes.Block) error {
// 	chainMu.Lock()
// 	defer chainMu.Unlock()
//
// 	if err := c.checkChain(chain...); err != nil {
// 		return fmt.Errorf("chain invalid: %s", err)
// 	}
// 	return c.Chain.ForkToNewChain(chain...)
// }

// PutNewChain calls ngchain's PutNewChain
func (pow *PoWork) PutNewChain(chain ...*ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := pow.checkChain(chain...); err != nil {
		return fmt.Errorf("chain invalid: %s", err)
	}

	return pow.chain.PutNewChain(chain...)
}

// PutNewBlock calls ngchain's PutNewBlock
func (pow *PoWork) PutNewBlock(block *ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := pow.checkBlock(block); err != nil {
		return err
	}

	return pow.chain.PutNewBlock(block)
}
