package blockchain

import (
	"bytes"
	"math/big"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// CheckBlock checks block before putting into chain.
func (chain *Chain) CheckBlock(b ngtypes.Block) error {
	block := b.(*ngtypes.FullBlock)

	return chain.View(func(txn *bbolt.Tx) error {
		return chain.checkBlockTxn(txn, block)
	})
}

// checkBlockTxn is the in-txn body of CheckBlock, so block import can
// check and apply within one write txn
func (chain *Chain) checkBlockTxn(txn *bbolt.Tx, block *ngtypes.FullBlock) error {
	if block.IsGenesis() {
		return nil
	}

	// check block itself
	if err := block.CheckError(); err != nil {
		return err
	}

	blockBucket := txn.Bucket(storage.BlockBucketName)

	originHash, err := ngblocks.GetOriginHash(blockBucket)
	if err != nil {
		return err
	}

	if !bytes.Equal(block.GetPrevHash(), originHash) {
		prevBlock, err := ngblocks.GetBlockByHash(blockBucket, block.GetPrevHash())
		if err != nil {
			return errors.Wrapf(err, "failed to get the prev block@%d %x",
				block.GetHeight()-1, block.GetPrevHash())
		}

		if err := checkBlockTarget(block, prevBlock); err != nil {
			return errors.Wrapf(err, "failed on checking block target")
		}
	}

	if err := validateUncles(storeGetBlock(blockBucket), block); err != nil {
		return errors.Wrapf(err, "failed on checking block uncles")
	}

	if err := ngstate.CheckBlockTxs(txn, block); err != nil {
		return errors.Wrap(err, "bactivate txs are invalid")
	}

	return nil
}

var ErrBlockTimeNotMonotonic = errors.New("block timestamp must be greater than its parent's")

func checkBlockTarget(block, prevBlock *ngtypes.FullBlock) error {
	// contracts read the timestamp, and the retarget depends on it:
	// without monotonicity a miner could freely manipulate both. The
	// future-drift bound lives in FullBlock.CheckError
	if block.BlockHeader.Timestamp <= prevBlock.BlockHeader.Timestamp {
		return errors.Wrapf(ErrBlockTimeNotMonotonic, "block@%d: %d <= parent %d",
			block.GetHeight(), block.BlockHeader.Timestamp, prevBlock.BlockHeader.Timestamp)
	}

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

	// burn-only base fee (ForkFeeMarket): the header's BaseFee must equal the
	// consensus-computed NextBaseFee from the parent, exactly like the
	// difficulty match. This holds BOTH pre- and post-fork — pre-fork
	// NextBaseFee returns MinBaseFee, so pre-fork blocks must carry MinBaseFee
	// (which they do by default). Parent is available here; genesis is exempt
	// (never reaches checkBlockTarget).
	correctBaseFee := ngtypes.NextBaseFee(
		block.BlockHeader.Network,
		prevBlock.GetHeight(),
		new(big.Int).SetBytes(prevBlock.BlockHeader.BaseFee),
		ngtypes.BlockUsedBytes(prevBlock),
	)
	blockBaseFee := new(big.Int).SetBytes(block.BlockHeader.BaseFee)
	if blockBaseFee.Cmp(correctBaseFee) != 0 {
		return errors.Wrapf(ngtypes.ErrBlockBaseFeeInvalid,
			"wrong base fee for block@%d, base fee in block: %s shall be %s",
			block.GetHeight(), blockBaseFee, correctBaseFee)
	}

	return nil
}
