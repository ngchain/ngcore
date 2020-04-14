package ngchain

import (
	"bytes"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// SwitchTo changes the items in db, requiring the first one of chain is an vault. The chain should follow the order vault0-block0-block1...-block6-vault2-block7...
// SwitchTo will override the origin data, using carefully
func (c *Chain) SwitchTo(chain ...*ngtypes.Block) error {
	log.Info("switching to new chain")
	/* Check Start */

	firstBlock := chain[0]
	if hash, _ := firstBlock.CalculateHash(); !bytes.Equal(hash, ngtypes.GenesisBlockHash) {
		// not genesis
		_, err := c.GetBlockByHash(firstBlock.GetPrevHash())
		if err != nil {
			return fmt.Errorf("the first block@%d's prevHash is invalid: %s", firstBlock.GetHeight(), err)
		}
	}

	/* Check End */

	/* Put start */
	err := c.db.Update(func(txn *badger.Txn) error {
		for i := 0; i < len(chain); i++ {
			block := chain[i]

			hash, _ := block.CalculateHash()
			raw, _ := utils.Proto.Marshal(block)
			log.Infof("putting block@%d: %x", block.Header.Height, hash)
			err := txn.Set(append(blockPrefix, hash...), raw)
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, utils.PackUint64LE(block.Header.Height)...), hash)
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, latestHeightTag...), utils.PackUint64LE(block.Header.Height))
			if err != nil {
				return err
			}
			err = txn.Set(append(blockPrefix, latestHashTag...), hash)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}
