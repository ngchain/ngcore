package ngblocks

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// PutNewBlock puts a new block into db and updates the tags.
// should check block before putting
// dev should continue upgrading the state after PutNewBlock
func PutNewBlock(txn *badger.Txn, block *ngtypes.Block) error {
	if block == nil {
		return fmt.Errorf("block is nil")
	}

	hash := block.Hash()

	err := checkBlock(txn, block.Height, block.PrevBlockHash)
	if err != nil {
		return err
	}

	log.Infof("putting block@%d: %x", block.Height, hash)
	err = PutBlock(txn, hash, block)
	if err != nil {
		return err
	}

	// put txs
	err = PutTxs(txn, block.Txs...)
	if err != nil {
		return err
	}

	// update helper
	err = PutLatestTags(txn, block.Height, hash)
	if err != nil {
		return err
	}

	return nil
}

func PutTxs(txn *badger.Txn, txs ...*ngtypes.Tx) error {
	for i := range txs {
		hash := txs[i].Hash()

		raw, err := utils.Proto.Marshal(txs[i])
		if err != nil {
			return err
		}

		err = txn.Set(append(txPrefix, hash...), raw)
		if err != nil {
			return err
		}
	}

	return nil
}

func PutBlock(txn *badger.Txn, hash []byte, block *ngtypes.Block) error {
	raw, err := utils.Proto.Marshal(block)
	if err != nil {
		return err
	}

	// put block hash & height
	err = txn.Set(append(blockPrefix, hash...), raw)
	if err != nil {
		return err
	}
	err = txn.Set(append(blockPrefix, utils.PackUint64LE(block.Height)...), hash)
	if err != nil {
		return err
	}

	return nil
}

func PutLatestTags(txn *badger.Txn, height uint64, hash []byte) error {
	err := txn.Set(append(blockPrefix, latestHeightTag...), utils.PackUint64LE(height))
	if err != nil {
		return err
	}
	err = txn.Set(append(blockPrefix, latestHashTag...), hash)
	if err != nil {
		return err
	}

	return nil
}
