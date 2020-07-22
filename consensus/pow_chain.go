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

	//TODO update miner work
	if pow.minerMod != nil {
		go func() {
			pow.minerMod.stop()
			pow.minerMod.start(pow.GetBlockTemplate())
		}()
	}

	return nil
}
