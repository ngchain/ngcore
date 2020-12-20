package ngchain

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/ngblocks"

	"github.com/ngchain/ngcore/ngtypes"
)

// GetTxByHash gets the tx with hash from db, so the tx must be applied.
func (chain *Chain) GetTxByHash(hash []byte) (*ngtypes.Tx, error) {
	var tx = &ngtypes.Tx{}

	if err := chain.View(func(txn *badger.Txn) error {
		var err error
		tx, err = ngblocks.GetTxByHash(txn, hash)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return tx, nil
}
