package ngblocks

import (
	"encoding/binary"
	"fmt"
	"github.com/c0mm4nd/rlp"

	"github.com/dgraph-io/badger/v3"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func GetTxByHash(txn *badger.Txn, hash []byte) (*ngtypes.Tx, error) {
	item, err := txn.Get(append(txPrefix, hash...))
	if err == badger.ErrKeyNotFound {
		return nil, err // export the keynotfound
	}
	if err != nil {
		return nil, err
	}
	raw, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	if hash == nil {
		return nil, fmt.Errorf("no such tx in hash")
	}

	var tx ngtypes.Tx
	err = rlp.DecodeBytes(raw, &tx)
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

func GetBlockByHash(txn *badger.Txn, hash []byte) (*ngtypes.Block, error) {

	key := append(blockPrefix, hash...)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, err // export the keynotfound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get item by key %x: %s", key, err)
	}
	raw, err := item.ValueCopy(nil)
	if err != nil {
		return nil, fmt.Errorf("no such block in hash %x: %s", hash, err)
	}

	var b ngtypes.Block
	err = rlp.DecodeBytes(raw, &b)
	if err != nil {
		return nil, err
	}

	return &b, nil
}

func GetBlockByHeight(txn *badger.Txn, height uint64) (*ngtypes.Block, error) {
	key := append(blockPrefix, utils.PackUint64LE(height)...)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, err // export the keynotfound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get item by key %x: %s", key, err)
	}
	hash, err := item.ValueCopy(nil)
	if err != nil || hash == nil {
		return nil, fmt.Errorf("no such block in height %d: %s", height, err)
	}
	key = append(blockPrefix, hash...)
	item, err = txn.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get item by key %x: %s", key, err)
	}
	raw, err := item.ValueCopy(nil)
	if err != nil || raw == nil {
		return nil, fmt.Errorf("no such block in hash %x: %s", hash, err)
	}

	var b ngtypes.Block
	err = rlp.DecodeBytes(raw, &b)
	if err != nil {
		return nil, err
	}

	return &b, nil
}

func GetLatestHeight(txn *badger.Txn) (uint64, error) {
	key := append(blockPrefix, latestHeightTag...)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return 0, err // export the keynotfound
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get latest height: %s", err)
	}
	raw, err := item.ValueCopy(nil)
	if err != nil {
		return 0, fmt.Errorf("no such height in latestTag: %s", err)
	}

	return binary.LittleEndian.Uint64(raw), nil
}

func GetLatestHash(txn *badger.Txn) ([]byte, error) {
	key := append(blockPrefix, latestHashTag...)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, err // export the keynotfound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest hash: %s", err)
	}
	hash, err := item.ValueCopy(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest hash: %s", err)
	}

	return hash, nil
}

func GetLatestBlock(txn *badger.Txn) (*ngtypes.Block, error) {
	hash, err := GetLatestHash(txn)
	if err != nil {
		return nil, err
	}

	block, err := GetBlockByHash(txn, hash)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func GetOriginHeight(txn *badger.Txn) (uint64, error) {
	key := append(blockPrefix, originHeightTag...)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return 0, err // export the keynotfound
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get latest height: %s", err)
	}
	raw, err := item.ValueCopy(nil)
	if err != nil {
		return 0, fmt.Errorf("no such height in latestTag: %s", err)
	}

	return binary.LittleEndian.Uint64(raw), nil
}

func GetOriginHash(txn *badger.Txn) ([]byte, error) {
	key := append(blockPrefix, originHashTag...)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, err // export the keynotfound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest hash: %s", err)
	}
	hash, err := item.ValueCopy(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest hash: %s", err)
	}

	return hash, nil
}

func GetOriginBlock(txn *badger.Txn) (*ngtypes.Block, error) {
	key := append(blockPrefix, originHashTag...)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, err // export the keynotfound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get origin block hash: %s", err)
	}
	hash, err := item.ValueCopy(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get origin block hash: %s", err)
	}

	item, err = txn.Get(append(blockPrefix, hash...))
	if err == badger.ErrKeyNotFound {
		return nil, err // export the keynotfound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get origin block hash: %s", err)
	}
	rawBlock, err := item.ValueCopy(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get origin block: %s", err)
	}

	var block ngtypes.Block
	err = rlp.DecodeBytes(rawBlock, &block)
	if err != nil {
		return nil, fmt.Errorf("failed to get origin block: %s", err)
	}

	return &block, nil
}
