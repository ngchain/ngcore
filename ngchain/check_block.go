package ngchain

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/dgraph-io/badger/v3"

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

		if !bytes.Equal(block.PrevBlockHash, originHash) {
			prevBlock, err := chain.GetBlockByHash(block.PrevBlockHash)
			if err != nil {
				return fmt.Errorf("failed to get the prev block@%d %x: %s", block.Height-1, block.PrevBlockHash, err)
			}

			if err := checkBlockTarget(block, prevBlock); err != nil {
				return err
			}
		}

		return ngstate.CheckBlockTxs(txn, block)
	})
	if err != nil {
		return fmt.Errorf("block txs are invalid: %s", err)
	}

	return nil
}

func checkBlockTarget(block, prevBlock *ngtypes.Block) error {
	correctDiff := ngtypes.GetNextDiff(block.Height, block.Timestamp, prevBlock)
	blockDiff := new(big.Int).SetBytes(block.Difficulty)
	actualDiff := block.GetActualDiff()

	if blockDiff.Cmp(correctDiff) != 0 {
		return fmt.Errorf("wrong block diff for block@%d, diff in block: %x shall be %x",
			block.GetHeight(), blockDiff, correctDiff)
	}

	if actualDiff.Cmp(correctDiff) < 0 {
		return fmt.Errorf("wrong block diff for block@%d, actual diff in block: %x shall be large than %x",
			block.GetHeight(), actualDiff, correctDiff)
	}

	return nil
}
