package blockchain

import (
	"bytes"
	"math/big"
	"sort"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngstate"
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

	// ErrUncleInvalid rejects a block whose uncle references break a
	// chain-context GHOST rule (wrong fork point, out of depth, on-chain,
	// double-referenced, or a wrong slot difficulty)
	ErrUncleInvalid = errors.New("invalid uncle reference")

	// ErrBranchNotHeavier rejects a converge switch to a branch that does not
	// carry strictly more cumulative work than the current tip (equal work is
	// resolved by the same smaller-hash tie-break as ApplyBlock). It keeps the
	// converge path's fork choice identical to gossip's, so a remote cannot
	// pull the node onto a lighter chain via a lucky checkpoint.
	ErrBranchNotHeavier = errors.New("converge branch is not heavier than the tip")
)

// validateUncles enforces the GHOST rules against the block's OWN ancestor
// chain, so the verdict is deterministic regardless of the current tip:
//   - each uncle forks off one of this block's ancestors at uncle.Height-1,
//     within [1, UncleMaxDepth] generations back;
//   - the uncle is not itself on this block's chain;
//   - the uncle's declared difficulty is correct for its slot and its
//     timestamp is monotonic against its parent (real, right-sized work);
//   - no uncle is referenced twice, nor by a recent ancestor.
//
// It runs BEFORE the block is weighed, so uncle work can never be counted
// for an unvalidated uncle. The context-free part (commitment, cap,
// standalone pow) is already done in FullBlock.CheckError.
// getBlockFn resolves a block by hash. In the single-block apply paths it
// reads the store; in the bulk branch-apply paths it also sees the
// not-yet-stored branch blocks, so an uncle whose parent is earlier in the
// same branch still resolves.
type getBlockFn func(hash []byte) (*ngtypes.FullBlock, error)

func storeGetBlock(blockBucket *bbolt.Bucket) getBlockFn {
	return func(hash []byte) (*ngtypes.FullBlock, error) {
		return ngblocks.GetBlockByHash(blockBucket, hash)
	}
}

// branchGetBlock resolves within an in-flight branch first, then the store.
func branchGetBlock(blockBucket *bbolt.Bucket, branch []*ngtypes.FullBlock) getBlockFn {
	byHash := make(map[string]*ngtypes.FullBlock, len(branch))
	for _, b := range branch {
		byHash[string(b.GetHash())] = b
	}
	return func(hash []byte) (*ngtypes.FullBlock, error) {
		if b, ok := byHash[string(hash)]; ok {
			return b, nil
		}
		return ngblocks.GetBlockByHash(blockBucket, hash)
	}
}

func validateUncles(getBlock getBlockFn, block *ngtypes.FullBlock) error {
	if len(block.Uncles) == 0 {
		return nil
	}

	// walk this block's ancestors deep enough to cover uncle parents and
	// the dedup window; collect them by height and note what they referenced
	ancestors := make(map[uint64]*ngtypes.FullBlock)
	referenced := make(map[string]struct{})
	lowest := uint64(0)
	if block.GetHeight() > ngtypes.UncleMaxDepth+1 {
		lowest = block.GetHeight() - (ngtypes.UncleMaxDepth + 1)
	}
	for cur := block; cur.GetHeight() > lowest; {
		parent, err := getBlock(cur.GetPrevHash())
		if err != nil {
			return errors.Wrapf(err, "uncle check: block@%d chain is not connected", block.GetHeight())
		}
		ancestors[parent.GetHeight()] = parent
		for _, u := range parent.Uncles {
			referenced[string(u.GetHash())] = struct{}{}
		}
		cur = parent
	}

	for _, u := range block.Uncles {
		depth := int64(block.GetHeight()) - int64(u.GetHeight())
		if depth < 1 || depth > int64(ngtypes.UncleMaxDepth) {
			return errors.Wrapf(ErrUncleInvalid, "uncle@%d is %d generations from block@%d (allowed 1..%d)",
				u.GetHeight(), depth, block.GetHeight(), ngtypes.UncleMaxDepth)
		}

		uncleParent, ok := ancestors[u.GetHeight()-1]
		if !ok || !bytes.Equal(uncleParent.GetHash(), u.GetPrevHash()) {
			return errors.Wrapf(ErrUncleInvalid, "uncle@%d does not fork off block@%d's chain",
				u.GetHeight(), block.GetHeight())
		}

		if onChain, ok := ancestors[u.GetHeight()]; ok && bytes.Equal(onChain.GetHash(), u.GetHash()) {
			return errors.Wrapf(ErrUncleInvalid, "uncle@%d is an ancestor of block@%d", u.GetHeight(), block.GetHeight())
		}

		if _, dup := referenced[string(u.GetHash())]; dup {
			return errors.Wrapf(ErrUncleInvalid, "uncle %x is already referenced by an ancestor", u.GetHash())
		}
		referenced[string(u.GetHash())] = struct{}{}

		if u.GetTimestamp() <= uncleParent.GetTimestamp() {
			return errors.Wrapf(ErrUncleInvalid, "uncle@%d timestamp is not monotonic", u.GetHeight())
		}
		correct := ngtypes.GetNextDiff(u.GetHeight(), u.GetTimestamp(), uncleParent)
		if new(big.Int).SetBytes(u.Difficulty).Cmp(correct) != 0 {
			return errors.Wrapf(ErrUncleInvalid, "uncle@%d declared difficulty %x is wrong for its slot (want %x)",
				u.GetHeight(), u.Difficulty, correct)
		}
	}

	return nil
}

// finalityHeight returns the height below which the canonical chain is
// FINAL for gossip-driven reorgs: the last checkpoint strictly below the
// tip. A checkpoint becomes immutable once a block is built on its round
func finalityHeight(tipHeight uint64) uint64 {
	if tipHeight == 0 {
		return 0
	}

	return (tipHeight - 1) / ngtypes.BlockCheckRound * ngtypes.BlockCheckRound
}

// sideBlockPruneHeight is the height below which side blocks may be dropped:
// the lower of the reorg finality line and the uncle-retention line (tip -
// UncleMaxDepth). Side blocks between the two are kept purely so a deep
// uncle stays referenceable, even though they can no longer win a reorg.
func sideBlockPruneHeight(tipHeight uint64) uint64 {
	fh := finalityHeight(tipHeight)
	if tipHeight <= ngtypes.UncleMaxDepth {
		return 0
	}
	if keep := tipHeight - ngtypes.UncleMaxDepth; keep < fh {
		return keep
	}
	return fh
}

// workOf returns the pow work one block contributes to its chain: ONLY its
// own declared difficulty. Uncle difficulty is deliberately NOT counted
// here. Folding it in (an earlier attempt) is unsound: validateUncles only
// proves an uncle is not on the nephew's OWN chain, so an attacker's fork
// could reference the honest chain's blocks as uncles and count that work
// on both chains, out-weighing honest with equal hashpower (a sub-50%
// reorg). Ethereum's GHOST keeps uncles out of the total-difficulty
// comparison for exactly this reason; uncles here are reward-only.
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

// CollectUncles gathers up to MaxUncles valid, not-yet-referenced uncle
// headers for a block that will extend the current canonical tip: recent
// side blocks that fork off the canonical chain within UncleMaxDepth
// generations and have not already been referenced by a recent ancestor.
// Best-effort — it returns whatever eligible set it finds (possibly none).
func (chain *Chain) CollectUncles() ([]*ngtypes.BlockHeader, error) {
	uncles := make([]*ngtypes.BlockHeader, 0, ngtypes.MaxUncles)

	err := chain.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		tipHeight, err := ngblocks.GetLatestHeight(blockBucket)
		if err != nil {
			return err
		}
		nextHeight := tipHeight + 1
		if nextHeight < 2 {
			return nil // an uncle needs height >= 1 and a canonical parent below it
		}

		minH := uint64(1)
		if nextHeight > ngtypes.UncleMaxDepth {
			minH = nextHeight - ngtypes.UncleMaxDepth
		}

		// what the recent canonical ancestors already referenced (dedup window)
		referenced := make(map[string]struct{})
		lowest := uint64(0)
		if nextHeight > ngtypes.UncleMaxDepth+1 {
			lowest = nextHeight - (ngtypes.UncleMaxDepth + 1)
		}
		for h := tipHeight; h > lowest; h-- {
			anc, err := ngblocks.GetBlockByHeight(blockBucket, h)
			if err != nil {
				return err
			}
			for _, u := range anc.Uncles {
				referenced[string(u.GetHash())] = struct{}{}
			}
		}

		candidates, err := ngblocks.ListSideBlocksInRange(blockBucket, minH, tipHeight)
		if err != nil {
			return err
		}
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].GetHeight() < candidates[j].GetHeight()
		})

		for _, c := range candidates {
			if len(uncles) >= ngtypes.MaxUncles {
				break
			}
			uh := c.GetHeight()

			// must fork off the canonical chain at uh-1 ...
			canonParent, err := ngblocks.GetBlockByHeight(blockBucket, uh-1)
			if err != nil || !bytes.Equal(canonParent.GetHash(), c.GetPrevHash()) {
				continue
			}
			// ... and must not itself be a canonical block
			if canonHere, err := ngblocks.GetBlockByHeight(blockBucket, uh); err == nil &&
				bytes.Equal(canonHere.GetHash(), c.GetHash()) {
				continue
			}
			if _, dup := referenced[string(c.GetHash())]; dup {
				continue
			}

			uncles = append(uncles, c.BlockHeader)
			referenced[string(c.GetHash())] = struct{}{}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return uncles, nil
}

// checkBranchBlock validates a branch block against ITS OWN parent
// (header + pow target + uncles); tx validity is enforced later by the
// state replay inside the reorg txn
func checkBranchBlock(getBlock getBlockFn, block, prev *ngtypes.FullBlock) error {
	if err := block.CheckError(); err != nil {
		return err
	}

	if block.GetHeight() != prev.GetHeight()+1 ||
		!bytes.Equal(block.GetPrevHash(), prev.GetHash()) {
		return ngblocks.ErrBranchDisconnected
	}

	if err := checkBlockTarget(block, prev); err != nil {
		return err
	}

	return validateUncles(getBlock, block)
}

// switchToBranchTxn atomically rewrites the canonical chain to the branch
// and replays the whole state; any failure aborts the txn leaving the
// old chain untouched
func (chain *Chain) switchToBranchTxn(txn *bbolt.Tx, branch []*ngtypes.FullBlock) error {
	blockBucket := txn.Bucket(storage.BlockBucketName)
	txBucket := txn.Bucket(storage.TxBucketName)

	// snapshot the logs of the blocks about to be orphaned, BEFORE anything
	// clears their receipts — a logs subscription notifies these as removed
	forkHeight := branch[0].GetHeight() - 1
	if oldTip, err := ngblocks.GetLatestHeight(blockBucket); err != nil {
		return err
	} else if oldTip > forkHeight {
		removed, err := ngstate.CollectLogsTxn(txn, ngstate.LogFilter{FromHeight: forkHeight + 1, ToHeight: oldTip})
		if err != nil {
			return err
		}
		chain.reorgRemoved = removed
	}

	// try to unwind state to the fork point from the changesets BEFORE the
	// block store switches (the unwind reads the old tip height). On an
	// archive node this replaces the full replay-from-genesis with an
	// O(reorg depth) revert; otherwise it reports false and we replay
	unwound, err := chain.State.UnwindToTxn(txn, forkHeight)
	if err != nil {
		return err
	}

	if err := ngblocks.SwitchToBranch(blockBucket, txBucket, branch); err != nil {
		return err
	}

	// the memoized work of the branch stays valid: it only depends on
	// prev links, which do not change on a canonical switch

	if unwound {
		// state is at the fork point: roll the branch forward
		if err := chain.State.ApplyBlocksTxn(txn, branch); err != nil {
			return err
		}
	} else if err := chain.State.RebuildFromBlockStoreTxn(txn); err != nil {
		return err
	}

	// the switch may land on a checkpoint tip: keep it servable and
	// reclaim the side blocks below the new finality line
	newTip := branch[len(branch)-1]
	if newTip.IsHead() {
		if err := chain.State.GenerateSnapshotTxn(txn); err != nil {
			return err
		}

		if !chain.State.Archive {
			if err := ngstate.PruneReceiptsTxn(txn, newTip.GetHeight()); err != nil {
				return err
			}
			if err := ngblocks.PruneAddrTxIndexTxn(txBucket, ngstate.ReceiptFloor(newTip.GetHeight())); err != nil {
				return err
			}
		}
		if _, err := ngblocks.PruneSideBlocks(blockBucket, sideBlockPruneHeight(newTip.GetHeight())); err != nil {
			return err
		}
	}

	// snapshot the logs the branch ADDS below its new tip: a logs
	// subscription replays them (the tip's own logs arrive via OnTipChanged,
	// so stop one short of it to avoid a duplicate)
	if newTip.GetHeight() > forkHeight+1 {
		added, err := ngstate.CollectLogsTxn(txn, ngstate.LogFilter{
			FromHeight: forkHeight + 1, ToHeight: newTip.GetHeight() - 1,
		})
		if err != nil {
			return err
		}
		chain.reorgAdded = added
	}

	return nil
}

// SwitchToBranch validates a connected branch fetched from a remote and
// atomically replaces the canonical chain with it (used by converging)
func (chain *Chain) SwitchToBranch(branch []*ngtypes.FullBlock) error {
	if len(branch) == 0 {
		return ngblocks.ErrBranchEmpty
	}

	chain.mu.Lock()
	defer chain.mu.Unlock()

	// drop any logs a previously aborted reorg txn left behind (see ApplyBlock)
	chain.reorgRemoved = nil
	chain.reorgAdded = nil
	err := chain.Update(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)

		forkParent, err := ngblocks.GetBlockByHash(blockBucket, branch[0].GetPrevHash())
		if err != nil {
			return errors.Wrap(err, "branch does not attach to any stored block")
		}

		getBlock := branchGetBlock(blockBucket, branch)
		prev := forkParent
		for _, block := range branch {
			if err := checkBranchBlock(getBlock, block, prev); err != nil {
				return err
			}
			prev = block
		}

		// converge switches only to a STRICTLY HEAVIER chain — the same fork
		// choice ApplyBlock uses. shouldConverge is only a trigger heuristic
		// (it ranks remotes by checkpoint luck, not cumulative work), so the
		// real safety lives here: reject a branch that is lighter, or loses the
		// equal-work tie-break, instead of switching onto a lighter chain.
		base, err := cumulativeWork(blockBucket, forkParent)
		if err != nil {
			return err
		}
		branchWork := new(big.Int).Set(base)
		for _, block := range branch {
			branchWork.Add(branchWork, workOf(block))
		}
		tip, err := ngblocks.GetLatestBlock(blockBucket)
		if err != nil {
			return err
		}
		tipWork, err := cumulativeWork(blockBucket, tip)
		if err != nil {
			return err
		}
		newTip := branch[len(branch)-1]
		if cmp := branchWork.Cmp(tipWork); cmp < 0 ||
			(cmp == 0 && bytes.Compare(newTip.GetHash(), tip.GetHash()) >= 0) {
			return errors.Wrapf(ErrBranchNotHeavier,
				"converge branch tip@%d (work %s) vs local tip@%d (work %s)",
				newTip.GetHeight(), branchWork, tip.GetHeight(), tipWork)
		}

		return chain.switchToBranchTxn(txn, branch)
	})
	if err != nil {
		return err
	}

	// removed/added logs first, so a subscription sees the rollback before
	// the new-head announcement (and its tip logs)
	chain.notifyReorg()
	chain.notifyTipChanged()

	return nil
}
