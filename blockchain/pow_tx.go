package blockchain

import (
	"github.com/c0mm4nd/dbolt"
	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// GetTxByHash gets the tx with hash from db, so the tx must be applied.
func (chain *Chain) GetTxByHash(hash []byte) (*ngtypes.FullTx, error) {
	tx := &ngtypes.FullTx{}

	if err := chain.View(func(txn *dbolt.Tx) error {
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
