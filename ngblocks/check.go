package ngblocks

import (
	"bytes"
	"errors"

	"go.etcd.io/bbolt"
	"github.com/ngchain/ngcore/storage"
)

var (
	ErrBlockHeightConflict = errors.New("already has a block on the same height")
	ErrPrevBlockNotExist   = errors.New("prev block does not exist")
)

func checkBlock(blockBucket *bbolt.Bucket, height uint64, prevHash []byte) error {
	if blockHeightExists(blockBucket, height) {
		return ErrBlockHeightConflict
	}

	if !blockPrevHashExists(blockBucket, height, prevHash) {
		return ErrPrevBlockNotExist
	}

	return nil
}

func blockHeightExists(blockBucket *bbolt.Bucket, height uint64) bool {
	if height == 0 {
		return true
	}
	_, err := GetBlockByHeight(blockBucket, height)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return false
	}

	if err != nil {
		log.Error(err)
	}

	return true
}

func blockPrevHashExists(blockBucket *bbolt.Bucket, height uint64, prevHash []byte) bool {
	if height == 0 && bytes.Equal(prevHash, make([]byte, 32)) {
		return true
	}

	b, err := GetBlockByHash(blockBucket, prevHash)
	if err != nil {
		return false
	}

	if b.GetHeight() == height-1 {
		return true
	}

	return false
}
