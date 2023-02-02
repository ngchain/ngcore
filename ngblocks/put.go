package ngblocks

import (
	"errors"

	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

var ErrPutEmptyBlock = errors.New("putting empty block into the db")

// PutNewBlock puts a new block into db and updates the tags.
// should check block before putting
// dev should continue upgrading the state after PutNewBlock
func PutNewBlock(blockBucket *bbolt.Bucket, txBucket *bbolt.Bucket, block *ngtypes.FullBlock) error {
	if block == nil {
		return ErrPutEmptyBlock
	}

	hash := block.GetHash()

	err := checkBlock(blockBucket, block.GetHeight(), block.GetPrevHash())
	if err != nil {
		return err
	}

	log.Infof("putting block@%d: %x", block.GetHeight(), hash)
	err = putBlock(blockBucket, hash, block)
	if err != nil {
		return err
	}

	// put txs
	err = putTxs(txBucket, block)
	if err != nil {
		return err
	}

	// update helper
	err = putLatestTags(blockBucket, block.GetHeight(), hash)
	if err != nil {
		return err
	}

	return nil
}

func putTxs(txBucket *bbolt.Bucket, block *ngtypes.FullBlock) error {
	for i := range block.Txs {
		hash := block.Txs[i].GetHash()

		raw, err := rlp.EncodeToBytes(block.Txs[i])
		if err != nil {
			return err
		}

		err = txBucket.Put(hash, raw)
		if err != nil {
			return err
		}
	}

	return nil
}

func putBlock(blockBucket *bbolt.Bucket, hash []byte, block *ngtypes.FullBlock) error {
	raw, err := rlp.EncodeToBytes(block)
	if err != nil {
		return err
	}

	// put block hash & height
	err = blockBucket.Put(hash, raw)
	if err != nil {
		return err
	}

	err = blockBucket.Put(utils.PackUint64LE(block.GetHeight()), hash)
	if err != nil {
		return err
	}

	return nil
}

func putLatestTags(blockBucket *bbolt.Bucket, height uint64, hash []byte) error {
	err := blockBucket.Put(storage.LatestHeightTag, utils.PackUint64LE(height))
	if err != nil {
		return err
	}
	err = blockBucket.Put(storage.LatestHashTag, hash)
	if err != nil {
		return err
	}

	return nil
}
