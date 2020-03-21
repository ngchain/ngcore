package chain

import "github.com/ngchain/ngcore/ngtypes"

func (c *Chain) MinedNewBlock(block *ngtypes.Block) error {
	err := c.PutNewBlock(block)
	if err != nil {
		return err
	}

	c.NewMinedBlockEvent <- block

	return nil
}

func (c *Chain) MinedNewVault(vault *ngtypes.Vault) error {
	err := c.PutNewVault(vault)
	if err != nil {
		return err
	}

	// for txpool
	c.NewVaultEvent <- vault

	return nil
}
