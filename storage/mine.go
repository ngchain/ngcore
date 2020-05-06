package storage

import "github.com/ngchain/ngcore/ngtypes"

// MinedNewBlock is called when **LOCAL PoW system** mined a new Block, putting it into db and then broadcast it.
func (c *Chain) MinedNewBlock(block *ngtypes.Block) error {
	err := c.PutNewBlock(block) // chain will verify the block
	if err != nil {
		return err
	}

	c.MinedBlockToP2PCh <- block
	c.MinedBlockToTxPoolCh <- block

	return nil
}
