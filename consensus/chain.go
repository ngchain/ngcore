package consensus

import (
	"github.com/ngchain/ngcore/ngtypes"
)

func (c *Consensus) InitWithChain(chain ...*ngtypes.Block) error {
	if err := c.checkChain(chain...); err != nil {
		return err
	}
	return c.Chain.InitWithChain(chain...)
}

func (c *Consensus) SwitchTo(chain ...*ngtypes.Block) error {
	if err := c.checkChain(chain...); err != nil {
		return err
	}
	return c.Chain.SwitchTo(chain...)
}

func (c *Consensus) PutNewChain(chain ...*ngtypes.Block) error {
	if err := c.checkChain(chain...); err != nil {
		return err
	}
	return c.Chain.PutNewChain(chain...)
}

func (c *Consensus) PutNewBlock(block *ngtypes.Block) error {
	if err := c.checkBlock(block); err != nil {
		return err
	}
	return c.Chain.PutNewBlock(block)
}
