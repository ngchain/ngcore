package ngblocks

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/ngchain/ngcore/ngtypes"
)

// ForcePutNewBlock puts a block into db regardless of local store check
// should check block self before putting
func (store *BlockStore) ForcePutNewBlock(txn *badger.Txn, block *ngtypes.Block) error {
	if block == nil {
		return fmt.Errorf("block is nil")
	}

	hash := block.Hash()

	// deleting txs
	if blockHeightExists(txn, block.Height) {
		b, err := GetBlockByHeight(txn, block.Height)
		if err != nil {
			return fmt.Errorf("failed to get existing block@%d: %s", block.Height, err)
		}

		err = delTxs(txn, b.Txs...)
		if err != nil {
			return fmt.Errorf("failed to del txs: %s", err)
		}
	}

	if !blockPrevHashExists(txn, block.Height, block.PrevBlockHash) {
		return fmt.Errorf("no prev block in storage: %x", block.GetPrevHash())
	}

	log.Infof("putting block@%d: %x", block.Height, hash)
	err := PutBlock(txn, hash, block)
	if err != nil {
		return fmt.Errorf("failed to pub block: %s", err)
	}

	// put txs
	err = PutTxs(txn, block.Txs...)
	if err != nil {
		return fmt.Errorf("failed to put txs: %s", err)
	}

	// update helper
	err = PutLatestTags(txn, block.Height, hash)
	if err != nil {
		return fmt.Errorf("failed to update tags: %s", err)
	}
	return nil
}

func delTxs(txn *badger.Txn, txs ...*ngtypes.Tx) error {
	for i := range txs {
		hash := txs[i].Hash()

		err := txn.Delete(append(txPrefix, hash...))
		if err != nil {
			return fmt.Errorf("failed to delete tx %x: %s", hash, err)
		}
	}

	return nil
}
