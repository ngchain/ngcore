package consensus

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
)

// checkBlock checks block before putting into chain.
func (c *PoWork) checkBlock(block *ngtypes.Block) error {
	if block.IsGenesis() {
		return nil
	}

	if err := block.CheckError(); err != nil {
		return err
	}

	prevHash := block.GetPrevHash()
	if !bytes.Equal(prevHash, ngtypes.GetGenesisBlockHash()) {
		prevBlock, err := c.chain.GetBlockByHash(prevHash)
		if err != nil {
			return err
		}

		if err := c.checkBlockTarget(block, prevBlock); err != nil {
			return err
		}
	}

	if err := c.sheetManager.CheckTxs(block.Txs...); err != nil {
		return err
	}

	return nil
}

func (c *PoWork) checkBlockTarget(block *ngtypes.Block, prevBlock *ngtypes.Block) error {
	correctDiff := ngtypes.GetNextDiff(prevBlock)
	actualDiff := block.GetActualDiff()

	if actualDiff.Cmp(correctDiff) < 0 {
		return fmt.Errorf("wrong block diff for block@%d, diff in block: %x shall be %x",
			block.GetHeight(), actualDiff, correctDiff)
	}

	return nil
}
