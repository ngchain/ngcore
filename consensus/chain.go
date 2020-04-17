package consensus

import (
	"fmt"
	"sync"

	"github.com/ngchain/ngcore/ngtypes"
)

var chainMu sync.Mutex

func (c *Consensus) InitWithChain(chain ...*ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := c.checkChain(chain...); err != nil {
		return fmt.Errorf("chain invalid: %s", err)
	}
	return c.Chain.InitWithChain(chain...)
}

func (c *Consensus) ForkToNewChain(chain ...*ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := c.checkChain(chain...); err != nil {
		return fmt.Errorf("chain invalid: %s", err)
	}
	return c.Chain.ForkToNewChain(chain...)
}

func (c *Consensus) PutNewChain(chain ...*ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := c.checkChain(chain...); err != nil {
		return fmt.Errorf("chain invalid: %s", err)
	}
	return c.Chain.PutNewChain(chain...)
}

func (c *Consensus) PutNewBlock(block *ngtypes.Block) error {
	chainMu.Lock()
	defer chainMu.Unlock()

	if err := c.checkBlock(block); err != nil {
		return err
	}
	return c.Chain.PutNewBlock(block)
}
