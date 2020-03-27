package ngchain

import "github.com/ngchain/ngcore/ngtypes"

func (c *Chain) MinedNewBlock(block *ngtypes.Block) error {
	err := c.PutNewBlock(block) // chain will verify the block
	if err != nil {
		return err
	}

	c.MinedBlockToP2PCh <- block
	c.MinedBlockToTxPoolCh <- block

	return nil
}

func (c *Chain) MinedNewVault(vault *ngtypes.Vault) error {
	err := c.PutNewVault(vault)
	if err != nil {
		return err
	}

	// for txpool
	c.NewVaultToTxPoolCh <- vault

	return nil
}
