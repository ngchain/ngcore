package storage

import (
	"bytes"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// ForcePutNewBlock puts a block into db regardless of local chain check
// should check block self before putting
func (c *Chain) ForcePutNewBlock(block *ngtypes.Block) error {
	if block == nil {
		return fmt.Errorf("block is nil")
	}

	hash := block.Hash()
	err := c.db.Update(func(txn *badger.Txn) error {
		if !bytes.Equal(hash, ngtypes.GetGenesisBlockHash()) {
			// when block is not genesis block, checking error
			if block.GetHeight() != 0 {
				if b, _ := c.GetBlockByHeight(block.GetHeight()); b != nil {
					if hashInDB := b.Hash(); bytes.Equal(hash, hashInDB) {
						return nil
					}

					// already has one
					err := delTxs(txn, b.Txs...)
					if err != nil {
						return err
					}
					// return fmt.Errorf("storage already has a block in same height: %s", string(jsonBlock))
				}
			}

			if _, err := c.GetBlockByHash(block.GetPrevHash()); err != nil {
				return fmt.Errorf("no prev block in storage: %x, %v", block.GetPrevHash(), err)
			}
		}

		raw, _ := utils.Proto.Marshal(block)
		log.Debugf("putting block@%d: %x", block.Height, hash)

		// put block hash & height
		err := txn.Set(append(blockPrefix, hash...), raw)
		if err != nil {
			return err
		}
		err = txn.Set(append(blockPrefix, utils.PackUint64LE(block.Height)...), hash)
		if err != nil {
			return err
		}

		// put txs
		err = putTxs(txn, block.Txs...)
		if err != nil {
			return err
		}

		// update helper
		err = txn.Set(append(blockPrefix, latestHeightTag...), utils.PackUint64LE(block.Height))
		if err != nil {
			return err
		}
		err = txn.Set(append(blockPrefix, latestHashTag...), hash)
		if err != nil {
			return err
		}
		return nil
	})

	return err
}

func delTxs(txn *badger.Txn, txs ...*ngtypes.Tx) error {
	for i := range txs {
		hash, _ := txs[i].CalculateHash()

		err := txn.Delete(append(txPrefix, hash...))
		if err != nil {
			return err
		}
	}

	return nil
}
