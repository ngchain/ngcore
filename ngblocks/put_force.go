package ngblocks

import (
	"github.com/c0mm4nd/dbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
)

// ForcePutNewBlock puts a block into db regardless of local store check
// should check block self before putting
func (store *BlockStore) ForcePutNewBlock(blockBucket *dbolt.Bucket, txBucket *dbolt.Bucket, block *ngtypes.Block) error {
	if block == nil {
		panic("block is nil")
	}

	hash := block.GetHash()

	// deleting txs
	if blockHeightExists(blockBucket, block.Header.Height) {
		b, err := GetBlockByHeight(blockBucket, block.Header.Height)
		if err != nil {
			return errors.Wrapf(err, "failed to get existing block@%d", block.Header.Height)
		}

		err = delTxs(txBucket, b.Txs...)
		if err != nil {
			return errors.Wrap(err, "failed to del txs")
		}
	}

	if !blockPrevHashExists(blockBucket, block.Header.Height, block.Header.PrevBlockHash) {
		return errors.Wrapf(ErrPrevBlockNotExist, "cannot find block %x", block.GetPrevHash())
	}

	log.Infof("putting block@%d: %x", block.Header.Height, hash)
	err := putBlock(blockBucket, hash, block)
	if err != nil {
		return errors.Wrap(err, "failed to put block")
	}

	// put txs
	err = putTxs(txBucket, block)
	if err != nil {
		return errors.Wrap(err, "failed to put txs")
	}

	// update helper
	err = putLatestTags(blockBucket, block.Header.Height, hash)
	if err != nil {
		return errors.Wrap(err, "failed to update tags")
	}
	return nil
}

func delTxs(txBucket *dbolt.Bucket, txs ...*ngtypes.Tx) error {
	for i := range txs {
		hash := txs[i].GetHash()

		err := txBucket.Delete(hash)
		if err != nil {
			return errors.Wrapf(err, "failed to delete tx %x", hash)
		}
	}

	return nil
}
