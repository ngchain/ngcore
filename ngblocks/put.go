package ngblocks

import (
	"errors"

	"github.com/c0mm4nd/dbolt"
	"github.com/c0mm4nd/rlp"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

var ErrPutEmptyBlock = errors.New("putting empty block into the db")

// PutNewBlock puts a new block into db and updates the tags.
// should check block before putting
// dev should continue upgrading the state after PutNewBlock
func PutNewBlock(blockBucket *dbolt.Bucket, txBucket *dbolt.Bucket, block *ngtypes.Block) error {
	if block == nil {
		return ErrPutEmptyBlock
	}

	hash := block.GetHash()

	err := checkBlock(blockBucket, block.Header.Height, block.Header.PrevBlockHash)
	if err != nil {
		return err
	}

	log.Infof("putting block@%d: %x", block.Header.Height, hash)
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
	err = putLatestTags(blockBucket, block.Header.Height, hash)
	if err != nil {
		return err
	}

	return nil
}

func putTxs(txBucket *dbolt.Bucket, block *ngtypes.Block) error {
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

func putBlock(blockBucket *dbolt.Bucket, hash []byte, block *ngtypes.Block) error {
	raw, err := rlp.EncodeToBytes(block)
	if err != nil {
		return err
	}

	// put block hash & height
	err = blockBucket.Put(hash, raw)
	if err != nil {
		return err
	}

	err = blockBucket.Put(utils.PackUint64LE(block.Header.Height), hash)
	if err != nil {
		return err
	}

	return nil
}

func putLatestTags(blockBucket *dbolt.Bucket, height uint64, hash []byte) error {
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
