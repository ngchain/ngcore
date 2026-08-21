package blockchain

import (
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"go.etcd.io/bbolt"
)

// GetTxByHash gets the tx with hash from db, so the tx must be applied.
func (chain *Chain) GetTxByHash(hash []byte) (*ngtypes.FullTx, error) {
	tx := &ngtypes.FullTx{}

	if err := chain.View(func(txn *bbolt.Tx) error {
		txBucket := txn.Bucket(storage.TxBucketName)

		var err error
		tx, err = ngblocks.GetTxByHash(txBucket, hash)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return tx, nil
}

// GetTxsByAddress returns the txs an address touched (as sender or
// recipient), in height order, within [fromHeight, toHeight] and capped at
// limit
func (chain *Chain) GetTxsByAddress(addr ngtypes.Address, fromHeight, toHeight uint64, limit int) ([]*ngtypes.FullTx, error) {
	var txs []*ngtypes.FullTx
	err := chain.View(func(txn *bbolt.Tx) error {
		var err error
		txs, err = ngblocks.GetTxsByAddress(txn.Bucket(storage.TxBucketName), addr, fromHeight, toHeight, limit)
		return err
	})

	return txs, err
}

// GetTxLocation resolves the block containing the tx via the tx index
func (chain *Chain) GetTxLocation(txHash []byte) (blockHash []byte, height uint64, err error) {
	err = chain.View(func(txn *bbolt.Tx) error {
		txBucket := txn.Bucket(storage.TxBucketName)
		blockBucket := txn.Bucket(storage.BlockBucketName)

		blockHash, err = ngblocks.GetTxBlockHash(txBucket, txHash)
		if err != nil {
			return err
		}

		block, err := ngblocks.GetBlockByHash(blockBucket, blockHash)
		if err != nil {
			return err
		}
		height = block.GetHeight()

		return nil
	})

	return blockHash, height, err
}
