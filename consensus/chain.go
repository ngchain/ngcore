package consensus

import (
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngtypes"
)

func (c *Consensus) InitWithChain(chain ...ngchain.Item) error {
	if err := c.checkChain(chain...); err != nil {
		return err
	}
	return c.Chain.InitWithChain(chain...)
}

func (c *Consensus) SwitchTo(chain ...ngchain.Item) error {
	if err := c.checkChain(chain...); err != nil {
		return err
	}
	return c.Chain.SwitchTo(chain...)
}

func (c *Consensus) PutNewChain(chain ...ngchain.Item) error {
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

func (c *Consensus) PutNewVault(vault *ngtypes.Vault) error {
	if err := c.checkVault(vault); err != nil {
		return err
	}
	return c.Chain.PutNewVault(vault)
}

func (c *Consensus) PutNewBlockWithVault(vault *ngtypes.Vault, block *ngtypes.Block) error {
	if err := c.checkChain([]ngchain.Item{vault, block}...); err != nil {
		return err
	}
	return c.Chain.PutNewBlockWithVault(vault, block)
}
