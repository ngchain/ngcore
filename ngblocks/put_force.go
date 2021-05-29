package ngblocks

import (
	"fmt"

	"github.com/dgraph-io/badger/v3"

	"github.com/ngchain/ngcore/ngtypes"
)

// ForcePutNewBlock puts a block into db regardless of local store check
// should check block self before putting
func (store *BlockStore) ForcePutNewBlock(txn *badger.Txn, block *ngtypes.Block) error {
	if block == nil {
		return fmt.Errorf("block is nil")
	}

	hash := block.GetHash()

	// deleting txs
	if blockHeightExists(txn, block.Header.Height) {
		b, err := GetBlockByHeight(txn, block.Header.Height)
		if err != nil {
			return fmt.Errorf("failed to get existing block@%d: %s", block.Header.Height, err)
		}

		err = delTxs(txn, b.Txs...)
		if err != nil {
			return fmt.Errorf("failed to del txs: %s", err)
		}
	}

	if !blockPrevHashExists(txn, block.Header.Height, block.Header.PrevBlockHash) {
		return fmt.Errorf("no prev block in storage: %x", block.GetPrevHash())
	}

	log.Infof("putting block@%d: %x", block.Header.Height, hash)
	err := putBlock(txn, hash, block)
	if err != nil {
		return fmt.Errorf("failed to pub block: %s", err)
	}

	// put txs
	err = putTxs(txn, block)
	if err != nil {
		return fmt.Errorf("failed to put txs: %s", err)
	}

	// update helper
	err = putLatestTags(txn, block.Header.Height, hash)
	if err != nil {
		return fmt.Errorf("failed to update tags: %s", err)
	}
	return nil
}

func delTxs(txn *badger.Txn, txs ...*ngtypes.Tx) error {
	for i := range txs {
		hash := txs[i].GetHash()

		err := txn.Delete(append(txPrefix, hash...))
		if err != nil {
			return fmt.Errorf("failed to delete tx %x: %s", hash, err)
		}
	}

	return nil
}
