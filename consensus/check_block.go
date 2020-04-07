package consensus

import (
	"fmt"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
)

// checkBlock checks block before putting into chain
func (c *Consensus) checkBlock(block *ngtypes.Block) error {
	if block == ngtypes.GetGenesisBlock() {
		return nil
	}

	if err := block.CheckError(); err != nil {
		return err
	}

	prevBlock, err := c.GetBlockByHash(block.GetPrevHash())
	if err != nil {
		return err
	}

	prevVault, err := c.GetVaultByHash(block.Header.PrevVaultHash)
	if err != nil {
		return err
	}

	if err = c.checkBlockTarget(block, prevBlock, prevVault); err != nil {
		return err
	}

	if err = c.TxPool.CheckTxs(block.Txs...); err != nil {
		return err
	}

	return nil
}

func (c *Consensus) checkBlockTarget(block *ngtypes.Block, prevBlock *ngtypes.Block, prevVault *ngtypes.Vault) error {
	correctTarget := ngtypes.GetNextTarget(prevBlock, prevVault)
	blockTarget := new(big.Int).SetBytes(block.Header.Target)
	if correctTarget.Cmp(blockTarget) != 0 {
		return fmt.Errorf("wrong block target for block: %v", block)
	}

	return nil
}
