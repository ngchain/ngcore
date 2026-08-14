package blockchain

import (
	"bytes"

	"go.etcd.io/bbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var (
	// ErrSheetMismatch occurs when the remote state sheet does not
	// describe the tip of the applied chain segment
	ErrSheetMismatch = errors.New("state sheet does not match the chain tip")
)

// ApplySnapshot atomically applies a remotely-fetched chain segment plus
// the state sheet of its tip: header/pow validation per block, canonical
// switch, work index and state reset all happen in ONE txn — a failure
// leaves the local chain untouched.
//
// TRUST NOTE: block headers carry no state commitment, so the sheet
// content itself cannot be verified cryptographically — snapshot mode
// trades tx-level verification for speed and trusts the serving peer
// for the state. Only the sheet's binding to the applied tip (hash,
// height, network) is enforced here
func (chain *Chain) ApplySnapshot(blocks []*ngtypes.FullBlock, sheet *ngtypes.Sheet) error {
	if len(blocks) == 0 {
		return ngblocks.ErrBranchEmpty
	}

	tip := blocks[len(blocks)-1]

	if sheet.Network != chain.Network ||
		sheet.Height != tip.GetHeight() ||
		!bytes.Equal(sheet.BlockHash, tip.GetHash()) {
		return errors.Wrapf(ErrSheetMismatch,
			"sheet %x@%d does not bind the segment tip %x@%d",
			sheet.BlockHash, sheet.Height, tip.GetHash(), tip.GetHeight())
	}

	err := chain.Update(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)
		txBucket := txn.Bucket(storage.TxBucketName)

		prev, err := ngblocks.GetBlockByHash(blockBucket, blocks[0].GetPrevHash())
		if err != nil {
			return errors.Wrap(err, "snapshot segment does not attach to any stored block")
		}

		for _, block := range blocks {
			if err := checkBranchBlock(block, prev); err != nil {
				return err
			}
			prev = block
		}

		if err := ngblocks.SwitchToBranch(blockBucket, txBucket, blocks); err != nil {
			return err
		}

		if _, err := cumulativeWork(blockBucket, tip); err != nil {
			return err
		}

		if err := chain.State.RebuildFromSheetTxn(txn, sheet); err != nil {
			return err
		}

		// the fetched sheet IS the tip's snapshot: keep it servable
		chain.State.SnapshotManager.PutSnapshot(tip.GetHeight(), tip.GetHash(), sheet)

		return nil
	})
	if err != nil {
		return err
	}

	chain.notifyTipChanged()

	return nil
}
