package consensus

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
)

// checkBlock checks block before putting into chain.
func (c *Consensus) checkBlock(block *ngtypes.Block) error {
	if block.IsGenesis() {
		return nil
	}

	if err := block.CheckError(); err != nil {
		return err
	}

	prevHash := block.GetPrevHash()
	if !bytes.Equal(prevHash, ngtypes.GetGenesisBlockHash()) {
		prevBlock, err := c.GetBlockByHash(prevHash)
		if err != nil {
			return err
		}

		if err := c.checkBlockTarget(block, prevBlock); err != nil {
			return err
		}
	}

	if err := c.SheetManager.CheckTxs(block.Txs...); err != nil {
		return err
	}

	return nil
}

func (c *Consensus) checkBlockTarget(block *ngtypes.Block, prevBlock *ngtypes.Block) error {
	correctTarget := ngtypes.GetNextTarget(prevBlock)
	blockTarget := new(big.Int).SetBytes(block.Header.Target)

	if correctTarget.Cmp(blockTarget) != 0 {
		return fmt.Errorf("wrong block target for block@%d, target in block: %x shall be %x",
			block.GetHeight(), blockTarget, correctTarget,
		)
	}

	return nil
}
