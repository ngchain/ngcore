package ngchain

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
)

// ApplyBlock checks the block and then calls ngchain's PutNewBlock, after which update the state
func (chain *Chain) ApplyBlock(block *ngtypes.Block) error {
	err := chain.Update(func(txn *badger.Txn) error {
		// check block first
		if err := chain.CheckBlock(block); err != nil {
			return err
		}

		// block is valid
		err := ngblocks.PutNewBlock(txn, block)
		if err != nil {
			return err
		}

		err = chain.State.Upgrade(txn, block) // handle Block Txs inside
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
