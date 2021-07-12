package ngblocks

import (
	"github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

// ForcePutNewBlock puts a block into db regardless of local store check
// should check block self before putting
func (store *BlockStore) ForcePutNewBlock(txn *badger.Txn, block *ngtypes.Block) error {
	if block == nil {
		panic("block is nil")
	}

	hash := block.GetHash()

	// deleting txs
	if blockHeightExists(txn, block.Header.Height) {
		b, err := GetBlockByHeight(txn, block.Header.Height)
		if err != nil {
			return errors.Wrapf(err, "failed to get existing block@%d", block.Header.Height)
		}

		err = delTxs(txn, b.Txs...)
		if err != nil {
			return errors.Wrap(err, "failed to del txs")
		}
	}

	if !blockPrevHashExists(txn, block.Header.Height, block.Header.PrevBlockHash) {
		return errors.Wrapf(ErrPrevBlockNotExist, "cannot find block %x", block.GetPrevHash())
	}

	log.Infof("putting block@%d: %x", block.Header.Height, hash)
	err := putBlock(txn, hash, block)
	if err != nil {
		return errors.Wrap(err, "failed to put block")
	}

	// put txs
	err = putTxs(txn, block)
	if err != nil {
		return errors.Wrap(err, "failed to put txs")
	}

	// update helper
	err = putLatestTags(txn, block.Header.Height, hash)
	if err != nil {
		return errors.Wrap(err, "failed to update tags")
	}
	return nil
}

func delTxs(txn *badger.Txn, txs ...*ngtypes.Tx) error {
	for i := range txs {
		hash := txs[i].GetHash()

		err := txn.Delete(append(txPrefix, hash...))
		if err != nil {
			return errors.Wrapf(err, "failed to delete tx %x", hash)
		}
	}

	return nil
}
