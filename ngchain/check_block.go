package ngchain

import (
	"bytes"
	"fmt"
	"github.com/dgraph-io/badger/v2"
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

	prevHash := block.GetPrevHash()
	if !bytes.Equal(prevHash, ngtypes.GetGenesisBlockHash(chain.Network)) {
		prevBlock, err := chain.GetBlockByHash(prevHash)
		if err != nil {
			return err
		}

		if err := checkBlockTarget(block, prevBlock); err != nil {
			return err
		}
	}

	err := chain.View(func(txn *badger.Txn) error {
		return ngstate.CheckTxs(txn, block.Txs...)
	})
	if err != nil {
		return err
	}

	return nil
}

func checkBlockTarget(block, prevBlock *ngtypes.Block) error {
	correctDiff := ngtypes.GetNextDiff(prevBlock)
	actualDiff := block.GetActualDiff()

	if actualDiff.Cmp(correctDiff) < 0 {
		return fmt.Errorf("wrong block diff for block@%d, diff in block: %x shall be %x",
			block.GetHeight(), actualDiff, correctDiff)
	}

	return nil
}
