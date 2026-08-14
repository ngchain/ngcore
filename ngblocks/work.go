package ngblocks

import (
	"math/big"

	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// blockWorkPrefix prefixes the key storing the cumulative pow work of the
// chain ending at a block hash
var blockWorkPrefix = []byte("work:")

func blockWorkKey(hash []byte) []byte {
	return append(blockWorkPrefix, hash...)
}

// PutBlockWork stores the cumulative work of the chain ending at the block
func PutBlockWork(blockBucket *bbolt.Bucket, hash []byte, work *big.Int) error {
	return blockBucket.Put(blockWorkKey(hash), work.Bytes())
}

// GetBlockWork loads the cumulative work of the chain ending at the block
func GetBlockWork(blockBucket *bbolt.Bucket, hash []byte) (*big.Int, error) {
	raw := blockBucket.Get(blockWorkKey(hash))
	if raw == nil {
		return nil, errors.Wrapf(storage.ErrKeyNotFound, "no work recorded for block %x", hash)
	}

	return new(big.Int).SetBytes(raw), nil
}

// PutSideBlock stores a block by its hash ONLY: the canonical height
// index, the tx index and the latest tags are left untouched. The block
// becomes reachable for a later reorg without being part of the main chain
func PutSideBlock(blockBucket *bbolt.Bucket, block *ngtypes.FullBlock) error {
	if block == nil {
		return ErrPutEmptyBlock
	}

	raw, err := rlp.EncodeToBytes(block)
	if err != nil {
		return err
	}

	return blockBucket.Put(block.GetHash(), raw)
}
