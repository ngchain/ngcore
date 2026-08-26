package ngstate

import (
	"math/big"
	"sync"

	logging "github.com/ngchain/zap-log"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

var log = logging.Logger("sheet")

// State is a global set of account & txs status
// (nil) --> B0(Prev: S0) --> B1(Prev: S1) -> B2(Prev: S2)
//
//	init (S0,S0)  -->   (S0,S1)  -->    (S1, S2)
type State struct {
	Network ngtypes.Network

	*bbolt.DB
	*SnapshotManager

	// Archive turns on historical-state retention: every block apply
	// captures each mutated address's pre-image (changesets + inverted
	// index), enabling the *AtHeight reads and, later, reorg unwind. Off
	// by default — no extra storage and no behavior change
	Archive bool

	// cs is the pre-image recorder of the block currently being applied.
	// Set for the duration of one Upgrade when Archive is on, nil
	// otherwise; the write helpers skip capture on a nil recorder
	cs *changeset
}

// InitStateFromSheet will initialize the state in the given db, with the sheet data
// this func is written for snapshot sync/converging when initializing from non-genesis
// checkpoint
func InitStateFromSheet(db *bbolt.DB, network ngtypes.Network, sheet *ngtypes.Sheet) *State {
	state := &State{
		DB:      db,
		Archive: true, // archive is the default startup mode
		SnapshotManager: &SnapshotManager{
			RWMutex:        sync.RWMutex{},
			heightToHash:   make(map[uint64]string),
			hashToSnapshot: make(map[string]*ngtypes.Sheet),
		},
	}
	err := state.Update(func(txn *bbolt.Tx) error {
		return initFromSheet(txn, sheet)
	})
	if err != nil {
		panic(err)
	}

	return state
}

// InitStateFromGenesis will initialize the state in the given db, with the default genesis sheet data
func InitStateFromGenesis(db *bbolt.DB, network ngtypes.Network) *State {
	state := &State{
		Network: network,
		DB:      db,
		Archive: true, // archive is the default startup mode: capture from genesis
		SnapshotManager: &SnapshotManager{
			RWMutex:        sync.RWMutex{},
			heightToHash:   make(map[uint64]string),
			hashToSnapshot: make(map[string]*ngtypes.Sheet),
		},
	}
	err := state.Update(func(txn *bbolt.Tx) error {
		// idempotent across restarts: a populated balance bucket means the db
		// was already seeded and the genesis block already applied. Re-running
		// initFromSheet + Upgrade(genesis) would credit GenesisAddress the
		// reward a second time (and, with the commitment now folded into the
		// header, that double-credit would fork the genesis hash). Skip.
		if k, _ := txn.Bucket(storage.Addr2BalBucketName).Cursor().First(); k != nil {
			return nil
		}

		err := initFromSheet(txn, ngtypes.GetGenesisSheet(network))
		if err != nil {
			return err
		}

		err = state.Upgrade(txn, ngtypes.GetGenesisBlock(network))
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		panic(err)
	}

	return state
}

// initFromSheet will overwrite a state from the given sheet
func initFromSheet(txn *bbolt.Tx, sheet *ngtypes.Sheet) error {
	addr2balBucket := txn.Bucket(storage.Addr2BalBucketName)

	for _, account := range sheet.Contracts {
		// setContract registers the module in the code bucket and
		// stores the slot referencing it by hash
		if err := setContract(txn, nil, account); err != nil {
			return err
		}
	}

	for _, balance := range sheet.Balances {
		err := addr2balBucket.Put(balance.Address[:], balance.Amount.Bytes())
		if err != nil {
			return err
		}
	}

	keyBucket := txn.Bucket(storage.KeyRegistryBucketName)
	for _, key := range sheet.Keys {
		if err := keyBucket.Put(key.Address[:], key.Entry); err != nil {
			return err
		}
	}

	return nil
}

// RebuildFromSheet will overwrite a state from the given sheet
func (state *State) RebuildFromSheet(sheet *ngtypes.Sheet) error {
	return state.Update(func(txn *bbolt.Tx) error {
		return state.RebuildFromSheetTxn(txn, sheet)
	})
}

// RebuildFromSheetTxn resets the state to the sheet INSIDE the given
// write txn, so a snapshot application can swap the chain and the state
// atomically
func (state *State) RebuildFromSheetTxn(txn *bbolt.Tx, sheet *ngtypes.Sheet) error {
	for _, name := range [][]byte{
		storage.Addr2BalBucketName,
		storage.ContractBucketName,
		storage.CodeBucketName,
		storage.KeyRegistryBucketName,
		storage.CommitBucketName, storage.CommitSpentBucketName,
		// the sheet jumps the state to its height with NO changesets below
		// it; drop any stale changeset/index entries from the pre-snapshot
		// chain so the coverage-based read guard treats sub-sheet heights as
		// unretained instead of surfacing an orphaned pre-image
		storage.BalChangeSetBucketName, storage.ContractChangeSetBucketName,
		storage.KeyChangeSetBucketName,
		storage.BalHistBucketName, storage.ContractHistBucketName,
	} {
		if err := txn.DeleteBucket(name); err != nil {
			return err
		}
		if _, err := txn.CreateBucket(name); err != nil {
			return err
		}
	}

	if err := initFromSheet(txn, sheet); err != nil {
		return err
	}
	// initFromSheet writes the buckets directly (not via the trie-aware
	// helpers), so rebuild the commitment from the fresh plain state
	return RebuildTrie(txn)
}

// RebuildFromBlockStore works for doing converge and remove all
func (state *State) RebuildFromBlockStore() error {
	return state.Update(state.RebuildFromBlockStoreTxn)
}

// RebuildFromBlockStoreTxn resets the state and replays the whole
// canonical chain INSIDE the given write txn, so a reorg can swap the
// chain and the state atomically: any failure aborts both
func (state *State) RebuildFromBlockStoreTxn(txn *bbolt.Tx) error {
	for _, name := range [][]byte{
		storage.Addr2BalBucketName,
		storage.ContractBucketName,
		storage.CodeBucketName,
		storage.KeyRegistryBucketName,
		storage.CommitBucketName, storage.CommitSpentBucketName,
		storage.ReceiptBucketName, // receipts regenerate with the replay
		// changesets/indices regenerate too: the replay re-applies every
		// block through Upgrade, which re-records them from scratch
		storage.BalChangeSetBucketName, storage.ContractChangeSetBucketName,
		storage.KeyChangeSetBucketName,
		storage.BalHistBucketName, storage.ContractHistBucketName,
	} {
		if err := txn.DeleteBucket(name); err != nil {
			return err
		}
		if _, err := txn.CreateBucket(name); err != nil {
			return err
		}
	}

	err := initFromSheet(txn, ngtypes.GetGenesisSheet(state.Network))
	if err != nil {
		return err
	}
	// reset the commitment to the seeded plain state; the block replay below
	// then maintains it incrementally through the trie-aware Upgrade path
	if err := RebuildTrie(txn); err != nil {
		return err
	}

	blockBucket := txn.Bucket(storage.BlockBucketName)
	latestHeight, err := ngblocks.GetLatestHeight(blockBucket)
	if err != nil {
		return err
	}

	for h := uint64(0); h <= latestHeight; h++ {
		b, err := ngblocks.GetBlockByHeight(blockBucket, h)
		if err != nil {
			return err
		}

		// the replayed chain may contain blocks which never passed the
		// canonical import path (a reorged-in side branch), so the full
		// tx-level checks (incl. the generate reward amount) run here
		if !b.IsGenesis() {
			if err := CheckBlockTxs(txn, b); err != nil {
				return errors.Wrapf(err, "invalid txs in replayed block@%d", h)
			}
		}

		if err := state.Upgrade(txn, b); err != nil {
			return err
		}
		// the replayed block must reproduce its committed post-state root
		if err := CheckStateRoot(txn, b.BlockHeader.StateRoot); err != nil {
			return errors.Wrapf(err, "replayed block@%d", h)
		}
	}

	return nil
}

// BackfillArchive rebuilds the changeset history from the block store when
// an archive node's db predates archiving — blocks are present but no
// changesets were recorded. It is a one-time, idempotent full replay
// (genesis -> tip) that recaptures state AND changesets, so historical
// reads and unwind work afterward. Reports whether a rebuild ran.
//
// A no-op when: archive is off, the changesets already cover the chain,
// or the node started from a snapshot (origin > 0) and so cannot replay
// below its origin — those keep whatever history they have from genesis
// forward, with the origin guard rejecting reads below it.
func (state *State) BackfillArchive() (bool, error) {
	if !state.Archive {
		return false, nil
	}

	var need bool
	err := state.View(func(txn *bbolt.Tx) error {
		blockBucket := txn.Bucket(storage.BlockBucketName)
		tip, err := ngblocks.GetLatestHeight(blockBucket)
		if err != nil {
			return err
		}
		origin, err := ngblocks.GetOriginHeight(blockBucket)
		if err != nil {
			return err
		}
		// only a genesis-origin (strict) node can replay the whole chain;
		// missing coverage at height 1 means the db predates archiving
		if origin == 0 && tip > 0 {
			need = !changesetCovers(txn, 1)
		}
		return nil
	})
	if err != nil || !need {
		return false, err
	}

	// this is a whole-chain replay in a single txn (bbolt buffers all dirty
	// pages until commit): expect it to be slow and memory-heavy on a large
	// chain. It runs once — later startups find full coverage and skip it
	log.Warn("archive backfill: replaying the chain to rebuild changeset history (one-time, may be slow)")
	if err := state.RebuildFromBlockStore(); err != nil {
		return false, err
	}
	return true, nil
}

// UnwindToTxn reverts the state from the current tip down to target using
// the recorded changesets — O(reorg depth) instead of replaying the whole
// chain. It reports whether the unwind was possible: false when archive is
// off or the changesets do not reach target (a snapshot-started node),
// leaving the caller to fall back to a full replay. The block store is
// NOT touched here
func (state *State) UnwindToTxn(txn *bbolt.Tx, target uint64) (bool, error) {
	if !state.Archive {
		return false, nil
	}

	tip, err := ngblocks.GetLatestHeight(txn.Bucket(storage.BlockBucketName))
	if err != nil {
		return false, err
	}
	if target >= tip {
		return true, nil // nothing to unwind
	}
	if !changesetCovers(txn, target+1) {
		return false, nil // history does not reach the fork point
	}

	for h := tip; h > target; h-- {
		unwindHeightTxn(txn, h)
	}

	// receipts are keyed by tx hash (not height), so unwindHeightTxn cannot
	// drop them per height; clear the whole reverted range here so the
	// branch re-apply rebuilds receipts fresh instead of doubling runs
	if err := deleteReceiptsAboveTxn(txn, target); err != nil {
		return false, err
	}

	return true, nil
}

// ApplyBlocksTxn applies a forward run of blocks (a reorg branch) onto the
// current state, enforcing tx validity like the replay path does
func (state *State) ApplyBlocksTxn(txn *bbolt.Tx, blocks []*ngtypes.FullBlock) error {
	for _, b := range blocks {
		if !b.IsGenesis() {
			if err := CheckBlockTxs(txn, b); err != nil {
				return errors.Wrapf(err, "invalid txs in branch block@%d", b.GetHeight())
			}
		}
		if err := state.Upgrade(txn, b); err != nil {
			return err
		}
		// a reorg branch block must reproduce its committed post-state root,
		// or the whole switch txn aborts and the old chain stays
		if err := CheckStateRoot(txn, b.BlockHeader.StateRoot); err != nil {
			return errors.Wrapf(err, "branch block@%d", b.GetHeight())
		}
	}

	return nil
}

// Upgrade will apply block's txs on current state
func (state *State) Upgrade(txn *bbolt.Tx, block *ngtypes.FullBlock) error {
	if state.Archive {
		// record every mutated address's pre-image under this height
		state.cs = newChangeset(block.GetHeight())
		defer func() { state.cs = nil }()
	}

	// apply this block's blind commitments FIRST (charge each committer's
	// fee, record heightLE‖Hash -> From), so a reveal in a LATER block can
	// find them. Genesis (height 0) carries none
	if err := state.handleCommits(txn, block); err != nil {
		return err
	}

	err := state.HandleTxs(txn, block.BlockHeader.Timestamp, block.Txs...)
	if err != nil {
		return err
	}

	// drop commitments unrevealed past the reveal window: the committer
	// forfeited its commit fee at commit time
	pruneCommits(txn, block.GetHeight())

	return nil
}

// handleCommits applies a block's commitments: each must verify and its
// committer must afford the fee, which is charged and recorded under the
// block height. Fails the block if any commitment is unaffordable or invalid.
func (state *State) handleCommits(txn *bbolt.Tx, block *ngtypes.FullBlock) error {
	for _, commit := range block.Commits {
		if err := commit.Verify(keyResolver(txn)); err != nil {
			return err
		}

		from, err := commit.From()
		if err != nil {
			return err
		}

		// single-inclusion (consensus): reject this committer's own commitment
		// if already pending on chain, so a re-heighted duplicate cannot double-
		// charge them. Keyed on (from, hash): a copycat reusing the public blind
		// hash has a different From and is neither blocked nor blocking.
		if commitFromPending(txn, from, commit.Hash, commit.Height) {
			return errors.Wrapf(ErrCommitDuplicate, "%s already committed %x", from, commit.Hash)
		}

		balance := getBalance(txn, from)
		if balance.Cmp(commit.Fee) < 0 {
			return errors.Wrapf(ErrCommitUnaffordable, "%s owes commit fee %s", from, commit.Fee)
		}
		if err := setBalance(txn, state.cs, from, new(big.Int).Sub(balance, commit.Fee)); err != nil {
			return err
		}

		if err := putCommit(txn, commit.Height, commit.Hash, from); err != nil {
			return err
		}
	}

	return nil
}
