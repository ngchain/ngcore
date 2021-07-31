package blockchain

import (
	"bytes"
	"math/big"

	"github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

// CheckBlock checks block before putting into chain.
func (chain *Chain) CheckBlock(block *ngtypes.Block) error {
	if block.IsGenesis() {
		return nil
	}

	// check block itself
	if err := block.CheckError(); err != nil {
		return err
	}

	err := chain.View(func(txn *badger.Txn) error {
		originHash, err := ngblocks.GetOriginHash(txn)
		if err != nil {
			panic(err)
		}

		if !bytes.Equal(block.Header.PrevBlockHash, originHash) {
			prevBlock, err := chain.GetBlockByHash(block.Header.PrevBlockHash)
			if err != nil {
				return errors.Wrapf(err, "failed to get the prev block@%d %x",
					block.Header.Height-1, block.Header.PrevBlockHash)
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

func checkBlockTarget(block, prevBlock *ngtypes.Block) error {
	correctDiff := ngtypes.GetNextDiff(block.Header.Height, block.Header.Timestamp, prevBlock)
	blockDiff := new(big.Int).SetBytes(block.Header.Difficulty)
	actualDiff := block.GetActualDiff()

	if blockDiff.Cmp(correctDiff) != 0 {
		return errors.Wrapf(ngtypes.ErrBlockDiffInvalid, "wrong block diff for block@%d, diff in block: %x shall be %x",
			block.Header.Height, blockDiff, correctDiff)
	}

	if actualDiff.Cmp(correctDiff) < 0 {
		return errors.Wrapf(ngtypes.ErrBlockDiffInvalid, "wrong block diff for block@%d, actual diff in block: %x shall be large than %x",
			block.Header.Height, actualDiff, correctDiff)
	}

	return nil
}
