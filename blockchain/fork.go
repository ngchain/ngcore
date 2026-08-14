package blockchain

import (
	"bytes"
	"math/big"

	"go.etcd.io/bbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

var (
	// ErrReorgBelowOrigin occurs when a fork does not attach to any block
	// this node stores (competing chain diverges below the origin block)
	ErrReorgBelowOrigin = errors.New("fork point is below the origin block")

	// ErrReorgBeyondFinality occurs when a gossip-driven reorg tries to
	// rewrite blocks below the rolling finality line. Deep switches must
	// go through the converging path, which ranks remote checkpoints
	ErrReorgBeyondFinality = errors.New("fork point is below the finality line")
)

// finalityHeight returns the height below which the canonical chain is
// FINAL for gossip-driven reorgs: the last checkpoint strictly below the
// tip. A checkpoint becomes immutable once a block is built on its round
func finalityHeight(tipHeight uint64) uint64 {
	if tipHeight == 0 {
		return 0
	}

	return (tipHeight - 1) / ngtypes.BlockCheckRound * ngtypes.BlockCheckRound
}

// workOf returns the pow work one block contributes to its chain:
// its declared (and checked) difficulty
func workOf(block *ngtypes.FullBlock) *big.Int {
	return new(big.Int).SetBytes(block.BlockHeader.Difficulty)
}

// cumulativeWork resolves the total work of the chain ending at block,
// walking prev links until a memoized value (or the origin) and storing
// the values back on the way, so the walk amortizes to O(1)
func cumulativeWork(blockBucket *bbolt.Bucket, block *ngtypes.FullBlock) (*big.Int, error) {
	pending := make([]*ngtypes.FullBlock, 0)
	cur := block

	var base *big.Int
	for {
		if work, err := ngblocks.GetBlockWork(blockBucket, cur.GetHash()); err == nil {
			base = work
			break
		}

		pending = append(pending, cur)

		originHash, err := ngblocks.GetOriginHash(blockBucket)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(cur.GetHash(), originHash) {
			base = big.NewInt(0) // the origin itself contributes via pending
			break
		}

		cur, err = ngblocks.GetBlockByHash(blockBucket, cur.GetPrevHash())
		if err != nil {
			return nil, errors.Wrapf(err, "chain of block %x is not connected", block.GetHash())
		}
	}

	work := new(big.Int).Set(base)
	for i := len(pending) - 1; i >= 0; i-- {
		work.Add(work, workOf(pending[i]))
		if err := ngblocks.PutBlockWork(blockBucket, pending[i].GetHash(), work); err != nil {
			return nil, err
		}
	}

	return work, nil
}

// isCanonical tells whether the block sits on the canonical height index
func isCanonical(blockBucket *bbolt.Bucket, block *ngtypes.FullBlock) bool {
	hash := blockBucket.Get(utils.PackUint64LE(block.GetHeight()))
	return hash != nil && bytes.Equal(hash, block.GetHash())
}

// collectBranch walks back from tip through side blocks until it reaches
// the canonical chain, returning the ascending branch since the fork point
func collectBranch(blockBucket *bbolt.Bucket, tip *ngtypes.FullBlock) ([]*ngtypes.FullBlock, error) {
	branch := make([]*ngtypes.FullBlock, 0)

	originHeight, err := ngblocks.GetOriginHeight(blockBucket)
	if err != nil {
		return nil, err
	}

	cur := tip
	for !isCanonical(blockBucket, cur) {
		branch = append([]*ngtypes.FullBlock{cur}, branch...)

		if cur.GetHeight() <= originHeight+1 {
			// the parent would sit at or below the origin: only the origin
			// itself can be the fork point
			originHash, err := ngblocks.GetOriginHash(blockBucket)
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(cur.GetPrevHash(), originHash) {
				return nil, ErrReorgBelowOrigin
			}
			return branch, nil
		}

		cur, err = ngblocks.GetBlockByHash(blockBucket, cur.GetPrevHash())
		if err != nil {
			return nil, errors.Wrap(err, "branch is not connected to the canonical chain")
		}
	}

	return branch, nil
}

// checkBranchBlock validates a branch block against ITS OWN parent
// (header + pow target); tx validity is enforced later by the state
// replay inside the reorg txn
func checkBranchBlock(block, prev *ngtypes.FullBlock) error {
	if err := block.CheckError(); err != nil {
		return err
	}

	if block.GetHeight() != prev.GetHeight()+1 ||
		!bytes.Equal(block.GetPrevHash(), prev.GetHash()) {
		return ngblocks.ErrBranchDisconnected
	}

	return checkBlockTarget(block, prev)
}

// switchToBranchTxn atomically rewrites the canonical chain to the branch
// and replays the whole state; any failure aborts the txn leaving the
// old chain untouched
func (chain *Chain) switchToBranchTxn(txn *bbolt.Tx, branch []*ngtypes.FullBlock) error {
	blockBucket := txn.Bucket(storage.BlockBucketName)
	txBucket := txn.Bucket(storage.TxBucketName)

	if err := ngblocks.SwitchToBranch(blockBucket, txBucket, branch); err != nil {
		return err
	}

	// the memoized work of the branch stays valid: it only depends on
	// prev links, which do not change on a canonical switch

	if err := chain.State.RebuildFromBlockStoreTxn(txn); err != nil {
		return err
	}

	// the switch may land on a checkpoint tip: keep it servable
	if branch[len(branch)-1].IsHead() {
		return chain.State.GenerateSnapshotTxn(txn)
	}

	return nil
}

// SwitchToBranch validates a connected branch fetched from a remote and
// atomically replaces the canonical chain with it (used by converging)
func (chain *Chain) SwitchToBranch(branch []*ngtypes.FullBlock) error {
	if len(branch) == 0 {
		return ngblocks.ErrBranchEmpty
	}

	err := chain.Update(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		prev, err := ngblocks.GetBlockByHash(blockBucket, branch[0].GetPrevHash())
		if err != nil {
			return errors.Wrap(err, "branch does not attach to any stored block")
		}

		for _, block := range branch {
			if err := checkBranchBlock(block, prev); err != nil {
				return err
			}
			prev = block
		}

		return chain.switchToBranchTxn(txn, branch)
	})
	if err != nil {
		return err
	}

	chain.notifyTipChanged()

	return nil
}
