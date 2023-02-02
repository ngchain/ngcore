package blockchain

import (
	"bytes"
	"math/big"

	"go.etcd.io/bbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// CheckBlock checks block before putting into chain.
func (chain *Chain) CheckBlock(b ngtypes.Block) error {
	block := b.(*ngtypes.FullBlock)
	if block.IsGenesis() {
		return nil
	}

	// check block itself
	if err := block.CheckError(); err != nil {
		return err
	}

	err := chain.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		originHash, err := ngblocks.GetOriginHash(blockBucket)
		if err != nil {
			panic(err)
		}

		if !bytes.Equal(block.GetPrevHash(), originHash) {
			prevBlock, err := chain.getBlockByHash(block.GetPrevHash())
			if err != nil {
				return errors.Wrapf(err, "failed to get the prev block@%d %x",
					block.GetHeight()-1, block.GetPrevHash())
			}

			if err := checkBlockTarget(block, prevBlock); err != nil {
				return errors.Wrapf(err, "failed on checking block target")
			}
		}

		return ngstate.CheckBlockTxs(txn, block)
	})
	if err != nil {
		return errors.Wrap(err, "block txs are invalid")
	}

	return nil
}

func checkBlockTarget(block, prevBlock *ngtypes.FullBlock) error {
	correctDiff := ngtypes.GetNextDiff(block.GetHeight(), block.BlockHeader.Timestamp, prevBlock)
	blockDiff := new(big.Int).SetBytes(block.BlockHeader.Difficulty)
	actualDiff := block.GetActualDiff()

	if blockDiff.Cmp(correctDiff) != 0 {
		return errors.Wrapf(ngtypes.ErrBlockDiffInvalid, "wrong block diff for block@%d, diff in block: %x shall be %x",
			block.GetHeight(), blockDiff, correctDiff)
	}

	if actualDiff.Cmp(correctDiff) < 0 {
		return errors.Wrapf(ngtypes.ErrBlockDiffInvalid, "wrong block diff for block@%d, actual diff in block: %x shall be large than %x",
			block.GetHeight(), actualDiff, correctDiff)
	}

	return nil
}
