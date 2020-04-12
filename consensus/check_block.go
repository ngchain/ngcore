package consensus

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
)

// checkBlock checks block before putting into chain
func (c *Consensus) checkBlock(block *ngtypes.Block) error {
	if err := block.CheckError(); err != nil {
		return err
	}

	genesisBlock := ngtypes.GetGenesisBlock()
	if block == genesisBlock {
		return nil
	}

	prevHash := block.GetPrevHash()
	if !bytes.Equal(prevHash, ngtypes.GenesisBlockHash) {
		prevBlock, err := c.GetBlockByHash(prevHash)
		if err != nil {
			return err
		}

		lastHeadBlock, err := c.GetBlockByHeight(block.GetHeight() - ngtypes.BlockCheckRound + 1)
		if err != nil {
			return err
		}
		if err = c.checkBlockTarget(block, prevBlock, lastHeadBlock); err != nil {
			return err
		}
	}

	if err := c.SheetManager.CheckCurrentTxs(block.Txs...); err != nil {
		return err
	}

	return nil
}

func (c *Consensus) checkBlockTarget(block *ngtypes.Block, prevBlock *ngtypes.Block, lastHeadBlock *ngtypes.Block) error {
	correctTarget := ngtypes.GetNextTarget(prevBlock, lastHeadBlock)
	blockTarget := new(big.Int).SetBytes(block.Header.Target)
	if correctTarget.Cmp(blockTarget) != 0 {
		return fmt.Errorf("wrong block target for block@%d, target in block: %x shall be %x", block.GetHeight(), blockTarget, correctTarget)
	}

	return nil
}
