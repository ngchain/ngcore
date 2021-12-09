package ngblocks

import (
	"encoding/binary"

	"github.com/c0mm4nd/dbolt"
	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

func GetTxByHash(txBucket *dbolt.Bucket, hash []byte) (*ngtypes.Tx, error) {
	rawTx := txBucket.Get(hash)
	if rawTx == nil {
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "no such tx in hash %x", hash)
	}

	var tx ngtypes.Tx
	err := rlp.DecodeBytes(rawTx, &tx)
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

func GetBlockByHash(blockBucket *dbolt.Bucket, hash []byte) (*ngtypes.Block, error) {
	rawBlock := blockBucket.Get(hash)
	if rawBlock == nil {
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "no such block in hash %x", hash)
	}

	var b ngtypes.Block
	err := rlp.DecodeBytes(rawBlock, &b)
	if err != nil {
		return nil, err
	}

	return &b, nil
}

func GetBlockByHeight(blockBucket *dbolt.Bucket, height uint64) (*ngtypes.Block, error) {
	key := utils.PackUint64LE(height)
	hash := blockBucket.Get(key)
	if hash == nil {
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "no such block in height %d", height)
	}

	rawBlock := blockBucket.Get(hash)
	if rawBlock == nil {
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "no such block in hash %x", hash)
	}

	var b ngtypes.Block
	err := rlp.DecodeBytes(rawBlock, &b)
	if err != nil {
		return nil, err
	}

	return &b, nil
}

func GetLatestHeight(blockBucket *dbolt.Bucket) (uint64, error) {
	height := blockBucket.Get(storage.LatestHeightTag)
	if height == nil {
		return 0, errors.Wrapf(storage.ErrKeyNotFound, "no such hash in latestTag")
	}

	return binary.LittleEndian.Uint64(height), nil
}

func GetLatestHash(blockBucket *dbolt.Bucket) ([]byte, error) {
	hash := blockBucket.Get(storage.LatestHashTag)
	if hash == nil {
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "no such hash in latestTag")
	}

	return hash, nil
}

func GetLatestBlock(blockBucket *dbolt.Bucket) (*ngtypes.Block, error) {
	hash, err := GetLatestHash(blockBucket)
	if err != nil {
		return nil, err
	}

	block, err := GetBlockByHash(blockBucket, hash)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func GetOriginHeight(blockBucket *dbolt.Bucket) (uint64, error) {
	height := blockBucket.Get(storage.OriginHeightTag)
	if height == nil {
		return 0, errors.Wrapf(storage.ErrKeyNotFound, "no such hash in originHeightTag")
	}

	return binary.LittleEndian.Uint64(height), nil
}

func GetOriginHash(blockBucket *dbolt.Bucket) ([]byte, error) {
	hash := blockBucket.Get(storage.OriginHashTag)
	if hash == nil {
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "no such hash in originHashTag")
	}

	return hash, nil
}

func GetOriginBlock(blockBucket *dbolt.Bucket) (*ngtypes.Block, error) {
	hash, err := GetOriginHash(blockBucket)
	if err != nil {
		return nil, err
	}

	block, err := GetBlockByHash(blockBucket, hash)
	if err != nil {
		return nil, err
	}

	return block, nil
}
