package ngblocks

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
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

// sideBlockPrefix indexes the stored side blocks (hash -> height) so
// pruning never has to scan the canonical blocks
var sideBlockPrefix = []byte("side:")

func sideBlockKey(hash []byte) []byte {
	return append(sideBlockPrefix, hash...)
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

	if err := blockBucket.Put(block.GetHash(), raw); err != nil {
		return err
	}

	return blockBucket.Put(sideBlockKey(block.GetHash()), utils.PackUint64LE(block.GetHeight()))
}

// PruneSideBlocks garbage-collects the side blocks (and their work
// entries) below the given height: below the finality line they can
// never win a reorg anymore
func PruneSideBlocks(blockBucket *bbolt.Bucket, belowHeight uint64) (pruned int, err error) {
	c := blockBucket.Cursor()

	victims := make([][]byte, 0)

	for k, v := c.Seek(sideBlockPrefix); k != nil && bytes.HasPrefix(k, sideBlockPrefix); k, v = c.Next() {
		height := binary.LittleEndian.Uint64(v)
		if height >= belowHeight {
			continue
		}

		hash := make([]byte, len(k)-len(sideBlockPrefix))
		copy(hash, k[len(sideBlockPrefix):])

		// defensive: never delete a body still referenced by the canonical
		// height index (a stale side mark on a promoted block would otherwise
		// orphan the index). This also self-heals dbs written before putBlock
		// learned to clear the side mark on promotion.
		if canonical := blockBucket.Get(utils.PackUint64LE(height)); bytes.Equal(canonical, hash) {
			continue
		}

		victims = append(victims, hash)
	}

	for _, hash := range victims {
		if err := blockBucket.Delete(hash); err != nil {
			return pruned, err
		}
		if err := blockBucket.Delete(blockWorkKey(hash)); err != nil {
			return pruned, err
		}
		if err := blockBucket.Delete(sideBlockKey(hash)); err != nil {
			return pruned, err
		}
		pruned++
	}

	return pruned, nil
}
