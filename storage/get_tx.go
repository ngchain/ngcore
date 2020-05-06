package storage

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// GetTxByHash gets the tx with hash from db, so the tx must be applied.
func (c *Chain) GetTxByHash(hash []byte) (*ngtypes.Tx, error) {
	var tx = &ngtypes.Tx{}

	if err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(txPrefix, hash...))
		if err != nil {
			return err
		}
		raw, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if hash == nil {
			return fmt.Errorf("no such tx in hash")
		}

		err = utils.Proto.Unmarshal(raw, tx)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return tx, nil
}
