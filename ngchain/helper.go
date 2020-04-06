package ngchain

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/ngtypes"
)

func (c *Chain) GetBlocksOnVaultHeight(vaultHeight uint64) ([]*ngtypes.Block, error) {
	var blocks []*ngtypes.Block
	tail := (vaultHeight+1)*ngtypes.BlockCheckRound - 1
	head := vaultHeight * ngtypes.BlockCheckRound

	for i := head; i <= tail; i++ {
		block, err := c.GetBlockByHeight(i)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				break
			} else {
				return nil, err
			}
		}

		blocks = append(blocks, block)
	}

	return blocks, nil
}
