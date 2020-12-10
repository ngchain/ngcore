package ngblocks

import (
	"bytes"
	"fmt"

	"github.com/dgraph-io/badger/v2"
)

func checkBlock(txn *badger.Txn, height uint64, prevHash []byte) error {
	if blockHeightExists(txn, height) {
		return fmt.Errorf("already has a block@%d", height)
	}

	if !blockPrevHashExists(txn, height, prevHash) {
		return fmt.Errorf("no prev block in ngblocks: %x", prevHash)
	}

	return nil
}

func blockHeightExists(txn *badger.Txn, height uint64) bool {
	if height == 0 {
		return true
	}
	_, err := GetBlockByHeight(txn, height)
	if err == badger.ErrKeyNotFound {
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

	if b.Height == height-1 {
		return true
	}

	return false
}
