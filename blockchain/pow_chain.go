package blockchain

import (
	"bytes"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// ApplyBlock imports a block with fork choice:
//   - a block extending the canonical tip is applied directly;
//   - a block on a side branch is stored by hash, and when its branch
//     carries more cumulative pow work than the canonical chain, the
//     node reorgs onto it atomically (chain index + full state replay
//     in one db txn — a failure rolls everything back)
func (chain *Chain) ApplyBlock(block *ngtypes.FullBlock) error {
	tipMoved := false
	err := chain.Update(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)
		txBucket := txn.Bucket(storage.TxBucketName)

		latestHash, err := ngblocks.GetLatestHash(blockBucket)
		if err != nil {
			return err
		}

		// fast path: the block extends the canonical tip
		if bytes.Equal(block.GetPrevHash(), latestHash) {
			if err := chain.checkBlockTxn(txn, block); err != nil {
				return err
			}

			if err := ngblocks.PutNewBlock(blockBucket, txBucket, block); err != nil {
				return err
			}

			if _, err := cumulativeWork(blockBucket, block); err != nil {
				return err
			}

			tipMoved = true
			if err := chain.State.Upgrade(txn, block); err != nil {
				return err
			}

			// checkpoint tips get a servable state snapshot, and the
			// side blocks below the new finality line get reclaimed
			if block.IsHead() {
				if err := chain.State.GenerateSnapshotTxn(txn); err != nil {
					return err
				}

				// archive nodes keep receipts so ng_getLogs reaches all history
				if !chain.State.Archive {
					if err := ngstate.PruneReceiptsTxn(txn, block.GetHeight()); err != nil {
						return err
					}
				}
				pruned, err := ngblocks.PruneSideBlocks(blockBucket, finalityHeight(block.GetHeight()))
				if err != nil {
					return err
				}
				if pruned > 0 {
					log.Warnf("pruned %d finalized side block(s)", pruned)
				}
			}

			return nil
		}

		// side path: the block forks off the canonical chain
		prev, err := ngblocks.GetBlockByHash(blockBucket, block.GetPrevHash())
		if err != nil {
			return errors.Wrapf(ngblocks.ErrPrevBlockNotExist,
				"orphan block@%d: unknown prev %x", block.GetHeight(), block.GetPrevHash())
		}

		if err := checkBranchBlock(block, prev); err != nil {
			return err
		}

		if err := ngblocks.PutSideBlock(blockBucket, block); err != nil {
			return err
		}

		blockWork, err := cumulativeWork(blockBucket, block)
		if err != nil {
			return err
		}

		tip, err := ngblocks.GetLatestBlock(blockBucket)
		if err != nil {
			return err
		}
		tipWork, err := cumulativeWork(blockBucket, tip)
		if err != nil {
			return err
		}

		if blockWork.Cmp(tipWork) <= 0 {
			log.Warnf("stored side block@%d %x (work %s <= tip %s), no reorg",
				block.GetHeight(), block.GetHash(), blockWork, tipWork)
			return nil
		}

		branch, err := collectBranch(blockBucket, block)
		if err != nil {
			return err
		}

		forkPoint := branch[0].GetHeight() - 1
		if forkPoint < finalityHeight(tip.GetHeight()) {
			return errors.Wrapf(ErrReorgBeyondFinality,
				"fork point@%d is below the finality line@%d",
				forkPoint, finalityHeight(tip.GetHeight()))
		}

		log.Warnf("reorg: switching to the heavier branch of %d block(s), fork point@%d, new tip@%d %x",
			len(branch), forkPoint, block.GetHeight(), block.GetHash())

		tipMoved = true
		return chain.switchToBranchTxn(txn, branch)
	})
	if err != nil {
		return err
	}

	if tipMoved {
		chain.notifyTipChanged()
	}

	return nil
}

// ForceApplyBlocks simply checks the block and then calls chain.ForcePutNewBlock
// but **do not** upgrade the state.
// so, after this, dev should do a regeneration or import the latest sheet.
func (chain *Chain) ForceApplyBlocks(blocks []*ngtypes.FullBlock) error {
	if err := chain.Update(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)
		txBucket := txn.Bucket(storage.TxBucketName)

		for i := 0; i < len(blocks); i++ {
			block := blocks[i]
			if err := block.CheckError(); err != nil {
				return err
			}

			err := chain.ForcePutNewBlock(blockBucket, txBucket, block)
			if err != nil {
				return errors.Wrap(err, "failed to force putting new block")
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
