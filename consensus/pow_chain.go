package consensus

import (
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// ApplyBlock checks the block and then calls ngchain's PutNewBlock, after which update the state
func (pow *PoWork) ApplyBlock(block *ngtypes.Block) error {
	pow.Lock()
	defer pow.Unlock()

	if err := pow.checkBlock(block); err != nil {
		return err
	}

	err := storage.GetChain().PutNewBlock(block)
	if err != nil {
		return err
	}

	err = ngstate.GetStateManager().UpgradeState(block) // handle Block Txs inside
	if err != nil {
		return err
	}

	return nil
}

// forceApplyBlocks checks the block and then calls ngchain's PutNewBlock, after which update the state
func (pow *PoWork) forceApplyBlocks(blocks []*ngtypes.Block) error {
	for i := 0; i < len(blocks); i++ {
		block := blocks[i]
		if err := pow.checkBlock(block); err != nil {
			return err
		}

		err := storage.GetChain().ForcePutNewBlock(block)
		if err != nil {
			return err
		}
	}

	return nil
}
