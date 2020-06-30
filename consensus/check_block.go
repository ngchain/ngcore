package consensus

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// checkBlock checks block before putting into chain.
func (pow *PoWork) checkBlock(block *ngtypes.Block) error {
	if block.IsGenesis() {
		return nil
	}

	if err := block.CheckError(); err != nil {
		return err
	}

	prevHash := block.GetPrevHash()
	if !bytes.Equal(prevHash, ngtypes.GetGenesisBlockHash()) {
		prevBlock, err := storage.GetChain().GetBlockByHash(prevHash)
		if err != nil {
			return err
		}

		if err := pow.checkBlockTarget(block, prevBlock); err != nil {
			return err
		}
	}

	err := ngstate.GetActiveState().CheckTxs(block.Txs...)
	if err != nil {
		return err
	}

	return nil
}

func (pow *PoWork) checkBlockTarget(block *ngtypes.Block, prevBlock *ngtypes.Block) error {
	correctDiff := ngtypes.GetNextDiff(prevBlock)
	actualDiff := block.GetActualDiff()

	if actualDiff.Cmp(correctDiff) < 0 {
		return fmt.Errorf("wrong block diff for block@%d, diff in block: %x shall be %x",
			block.GetHeight(), actualDiff, correctDiff)
	}

	return nil
}
