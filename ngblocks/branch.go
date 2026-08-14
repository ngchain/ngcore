package ngblocks

import (
	"bytes"

	"go.etcd.io/bbolt"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var (
	ErrBranchEmpty        = errors.New("branch is empty")
	ErrBranchDisconnected = errors.New("branch blocks are not connected")
	ErrBranchDetached     = errors.New("branch does not attach to the canonical chain")
)

// SwitchToBranch rewrites the canonical chain to the given branch: the
// branch must be ascending and connected, and its first block must build
// on a block of the current canonical chain (the fork ancestor).
//
// Replaced canonical blocks stay stored by hash (they become side
// blocks), but their txs leave the tx index; stale canonical heights
// above the new tip are unlinked as well
func SwitchToBranch(blockBucket, txBucket *bbolt.Bucket, branch []*ngtypes.FullBlock) error {
	if len(branch) == 0 {
		return ErrBranchEmpty
	}

	// the branch must connect internally...
	for i := 1; i < len(branch); i++ {
		if branch[i].GetHeight() != branch[i-1].GetHeight()+1 ||
			!bytes.Equal(branch[i].GetPrevHash(), branch[i-1].GetHash()) {
			return ErrBranchDisconnected
		}
	}

	// ...and attach to the canonical chain right below its first block
	ancestorHeight := branch[0].GetHeight() - 1
	ancestorHash := blockBucket.Get(utils.PackUint64LE(ancestorHeight))
	if ancestorHash == nil || !bytes.Equal(ancestorHash, branch[0].GetPrevHash()) {
		return errors.Wrapf(ErrBranchDetached, "no canonical ancestor@%d %x",
			ancestorHeight, branch[0].GetPrevHash())
	}

	oldHeight, err := GetLatestHeight(blockBucket)
	if err != nil {
		return err
	}

	// unlink the txs of every canonical block being replaced or truncated
	for h := branch[0].GetHeight(); h <= oldHeight; h++ {
		old, err := GetBlockByHeight(blockBucket, h)
		if err != nil {
			return errors.Wrapf(err, "broken canonical chain at height %d", h)
		}

		if err := delTxs(txBucket, old.Txs...); err != nil {
			return err
		}
	}

	// connect the branch
	for _, block := range branch {
		if err := putBlock(blockBucket, block.GetHash(), block); err != nil {
			return err
		}
		if err := putTxs(txBucket, block); err != nil {
			return err
		}
	}

	// drop stale canonical heights above the new tip
	newTip := branch[len(branch)-1]
	for h := newTip.GetHeight() + 1; h <= oldHeight; h++ {
		if err := blockBucket.Delete(utils.PackUint64LE(h)); err != nil {
			return err
		}
	}

	return putLatestTags(blockBucket, newTip.GetHeight(), newTip.GetHash())
}
