package consensus

import (
	"fmt"
	"sync"

	"github.com/ngchain/ngcore/ngtypes"
)

var chainMu sync.Mutex

func (c *PoWork) InitWithChain(chain ...*ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := c.checkChain(chain...); err != nil {
		return fmt.Errorf("chain invalid: %s", err)
	}

	return c.chain.InitWithChain(chain...)
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

func (c *PoWork) PutNewChain(chain ...*ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := c.checkChain(chain...); err != nil {
		return fmt.Errorf("chain invalid: %s", err)
	}

	return c.chain.PutNewChain(chain...)
}

func (c *PoWork) PutNewBlock(block *ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := c.checkBlock(block); err != nil {
		return err
	}

	return c.chain.PutNewBlock(block)
}
