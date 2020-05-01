package consensus

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
)

// checkChain is a helper to check whether the items are aligned as a chain.
func (c *PoWork) checkChain(blocks ...*ngtypes.Block) error {
	if len(blocks) == 0 {
		return fmt.Errorf("empty chain")
	}

	if len(blocks) == 1 {
		return c.checkBlock(blocks[0])
	}

	var curBlock, prevBlock *ngtypes.Block

	var prevBlockHash, curBlockHash []byte

	var err error

	firstBlock := blocks[0]

	if !firstBlock.IsGenesis() {
		prevBlock, err = c.chain.GetBlockByHash(firstBlock.GetPrevHash())
		if err != nil {
			return fmt.Errorf("failed to init prevBlock %x from db: %s", firstBlock.GetPrevHash(), err)
		}

		prevBlockHash, _ = prevBlock.CalculateHash()
	}

	for i := 0; i < len(blocks); i++ {
		curBlock = blocks[i]
		if err := curBlock.CheckError(); err != nil {
			return err
		}

		if prevBlock != nil {
			if err := c.checkBlockTarget(curBlock, prevBlock); err != nil {
				return err
			}
		}

		if err := c.sheetManager.CheckTxs(curBlock.Txs...); err != nil {
			return err
		}

		curBlockHash, _ = curBlock.CalculateHash()

		if !bytes.Equal(prevBlockHash, curBlock.GetPrevHash()) {
			return fmt.Errorf("block@%d:%x 's prevBlockHash: %x is not matching block@%d:%x 's hash",
				curBlock.GetHeight(), curBlockHash, curBlock.GetPrevHash(), prevBlock.GetHeight(), prevBlockHash)
		}

		prevBlock = curBlock
		prevBlockHash = curBlockHash
	}

	return nil
}
