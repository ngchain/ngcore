package ngblocks

import (
	"bytes"
	"errors"

	"github.com/dgraph-io/badger/v3"
)

var (
	ErrBlockHeightConflict = errors.New("already has a block on the same height")
	ErrPrevBlockNotExist   = errors.New("prev block does not exist")
)

func checkBlock(txn *badger.Txn, height uint64, prevHash []byte) error {
	if blockHeightExists(txn, height) {
		return ErrBlockHeightConflict
	}

	if !blockPrevHashExists(txn, height, prevHash) {
		return ErrPrevBlockNotExist
	}

	return nil
}

func blockHeightExists(txn *badger.Txn, height uint64) bool {
	if height == 0 {
		return true
	}
	_, err := GetBlockByHeight(txn, height)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false
	}

	if err != nil {
		log.Error(err)
	}

	return true
}

func blockPrevHashExists(txn *badger.Txn, height uint64, prevHash []byte) bool {
	if height == 0 && bytes.Equal(prevHash, make([]byte, 32)) {
		return true
	}

	b, err := GetBlockByHash(txn, prevHash)
	if err != nil {
		return false
	}

	if b.Header.Height == height-1 {
		return true
	}

	return false
}
