package blockchain

import (
	"go.etcd.io/bbolt"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// ApplyBlock checks the block and then calls blockchain's PutNewBlock, and then auto-upgrade the state.
func (chain *Chain) ApplyBlock(block *ngtypes.FullBlock) error {
	err := chain.Update(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)
		txBucket := txn.Bucket(storage.BlockBucketName)

		// check block first
		if err := chain.CheckBlock(block); err != nil {
			return err
		}

		// block is valid
		err := ngblocks.PutNewBlock(blockBucket, txBucket, block)
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
