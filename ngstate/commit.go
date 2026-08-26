package ngstate

import (
	"bytes"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// CheckCommitment validates a commitment for pool admission at the given
// height: it must be well-formed (32B Hash, non-nil Fee, matching Height), its
// signature must verify against the on-chain key registry, and its committer
// must currently afford the fee.
func CheckCommitment(txn *bbolt.Tx, commit *ngtypes.Commitment, height uint64) error {
	if err := commit.CheckError(height, keyResolver(txn)); err != nil {
		return err
	}

	from, err := commit.From()
	if err != nil {
		return err
	}

	// single-inclusion: this committer's own commitment, already pending on
	// chain, cannot be re-heighted and charged again. Keyed on (from, hash) so a
	// copycat that reuses the public blind hash neither blocks nor is blocked.
	if commitFromPending(txn, from, commit.Hash, height) {
		return errors.Wrapf(ErrCommitDuplicate, "%s already committed %x", from, commit.Hash)
	}

	if getBalance(txn, from).Cmp(commit.Fee) < 0 {
		return errors.Wrapf(ErrCommitUnaffordable, "%s owes commit fee %s", from, commit.Fee)
	}

	return nil
}

// commitKey is the height-major store key: heightLE(8) ‖ Hash(32). A prefix
// cursor over one height yields every commitment recorded at it (used by the
// per-height block-undo and by pruning).
func commitKey(height uint64, hash []byte) []byte {
	key := make([]byte, 8+ngtypes.HashSize)
	copy(key[:8], utils.PackUint64LE(height))
	copy(key[8:], hash)
	return key
}

// putCommit records a commitment at the given height: heightLE‖Hash -> From.
func putCommit(txn *bbolt.Tx, height uint64, hash []byte, from ngtypes.Address) error {
	key := commitKey(height, hash)
	if err := txn.Bucket(storage.CommitBucketName).Put(key, from[:]); err != nil {
		return err
	}
	// mirror the pending commitment into the state commitment
	trieSetCommit(txn, key, from[:])
	return nil
}

// findCommit looks up an UNREVEALED commitment by (From, Hash) that was
// recorded at some height h in [reveal-CommitWindow, reveal). It returns the
// recording height and whether one was found. The strict h < reveal bound is
// the load-bearing anti-same-block-reaction rule.
func findCommit(txn *bbolt.Tx, from ngtypes.Address, hash []byte, revealHeight uint64) (uint64, bool) {
	bucket := txn.Bucket(storage.CommitBucketName)

	var low uint64
	if revealHeight > ngtypes.CommitWindow {
		low = revealHeight - ngtypes.CommitWindow
	}

	for h := low; h < revealHeight; h++ {
		v := bucket.Get(commitKey(h, hash))
		if v != nil && bytes.Equal(v, from[:]) {
			return h, true
		}
	}

	return 0, false
}

// consumeCommit deletes a matched commitment (a reveal spends it).
func consumeCommit(txn *bbolt.Tx, height uint64, hash []byte) error {
	key := commitKey(height, hash)
	if err := txn.Bucket(storage.CommitBucketName).Delete(key); err != nil {
		return err
	}
	// drop the pending-commitment leaf from the state commitment
	trieSetCommit(txn, key, nil)
	return nil
}

// commitFromPending reports whether the SAME committer (from) already has a
// commitment with this hash recorded (unspent) on chain below `height`, within
// the window. It is the cross-block de-duplication guard: a commitment now signs
// a height-INDEPENDENT digest (so a node may relay it to a later block), which
// would otherwise let the SAME signed commitment be re-heighted and re-included
// at several heights, charging its committer the fee each time. Making the
// committer's own re-heighted duplicate invalid caps their commitment at ONE
// on-chain inclusion.
//
// It deliberately keys on (from, hash), NOT hash alone. A commitment's blind
// hash is PUBLIC in gossip and attacker-choosable; a third party that copies it
// signs with a DIFFERENT From, so it must neither block this committer nor be
// blocked by them — their reveals match by (from, hash) independently
// (findCommit). Keying on hash alone would let a copycat commitment censor the
// victim's real one. A commitment consumed by a reveal is gone from the bucket,
// so re-committing the same content after it was revealed stays allowed.
func commitFromPending(txn *bbolt.Tx, from ngtypes.Address, hash []byte, height uint64) bool {
	bucket := txn.Bucket(storage.CommitBucketName)

	// scan one past the window so an as-yet-unpruned pending commit is still seen
	var low uint64
	if height > ngtypes.CommitWindow+1 {
		low = height - ngtypes.CommitWindow - 1
	}
	for h := low; h < height; h++ {
		if v := bucket.Get(commitKey(h, hash)); v != nil && bytes.Equal(v, from[:]) {
			return true
		}
	}
	return false
}

// CommitOnChain reports whether the committer's own commitment (from, hash) is
// already recorded on chain as of the next block `next`. The commit relay uses
// it to stop re-submitting a commitment once it has landed, so it is never
// double-included.
func CommitOnChain(txn *bbolt.Tx, from ngtypes.Address, hash []byte, next uint64) bool {
	return commitFromPending(txn, from, hash, next)
}

// spentKey is the consumption-journal key: revealHeightLE(8) ‖ Hash(32).
func spentKey(revealHeight uint64, hash []byte) []byte {
	key := make([]byte, 8+ngtypes.HashSize)
	copy(key[:8], utils.PackUint64LE(revealHeight))
	copy(key[8:], hash)
	return key
}

// journalConsumed records that a reveal at revealHeight spent the commitment
// (hash, from) recorded at recordHeight, so a block-undo of revealHeight can
// re-put it. Value = recordHeightLE(8) ‖ From(32).
func journalConsumed(txn *bbolt.Tx, revealHeight, recordHeight uint64, hash []byte, from ngtypes.Address) error {
	val := make([]byte, 8+ngtypes.AddressSize)
	copy(val[:8], utils.PackUint64LE(recordHeight))
	copy(val[8:], from[:])
	return txn.Bucket(storage.CommitSpentBucketName).Put(spentKey(revealHeight, hash), val)
}

// restoreConsumedAtHeight undoes every consumption a reveal block at height h
// performed: it re-puts each spent commitment at its ORIGINAL recording height
// and clears the journal for h. This is what lets a reorg drop a reveal block
// while its commitment's recording block stays canonical below the fork point.
func restoreConsumedAtHeight(txn *bbolt.Tx, h uint64) {
	spent := txn.Bucket(storage.CommitSpentBucketName)
	if spent == nil {
		return
	}
	prefix := utils.PackUint64LE(h)

	type entry struct{ hash, val []byte }
	var entries []entry
	c := spent.Cursor()
	for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
		entries = append(entries, entry{append([]byte{}, k[8:]...), append([]byte{}, v...)})
	}
	for _, e := range entries {
		recordHeight := utils.UnpackUint64LE(e.val[:8])
		var from ngtypes.Address
		copy(from[:], e.val[8:])
		_ = putCommit(txn, recordHeight, e.hash, from)
		_ = spent.Delete(spentKey(h, e.hash))
	}
}

// deleteCommitsAtHeight drops every commitment recorded at height h. Wired
// into the per-height reorg undo so an unwound block's commitments vanish.
func deleteCommitsAtHeight(txn *bbolt.Tx, h uint64) {
	bucket := txn.Bucket(storage.CommitBucketName)
	if bucket == nil {
		return
	}
	prefix := utils.PackUint64LE(h)

	var keys [][]byte
	c := bucket.Cursor()
	for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
		keys = append(keys, append([]byte{}, k...))
	}
	for _, k := range keys {
		_ = bucket.Delete(k)
		// this IS the mempool:commit bucket, so drop its trie leaf too
		trieSetCommit(txn, k, nil)
	}
}

// pruneCommits drops every commitment recorded — and every consumption
// journaled — at a height below tip-CommitWindow. Unrevealed past the window,
// the committer already forfeited its commit fee; a reorg that deep would be
// unwinding heights whose commitments are pruned anyway, so the restore
// journal is no longer needed either.
func pruneCommits(txn *bbolt.Tx, tip uint64) {
	if tip <= ngtypes.CommitWindow {
		return
	}
	cutoff := tip - ngtypes.CommitWindow // delete heights strictly below this
	// only the mempool:commit bucket is in the state commitment; its prune
	// must drop the matching trie leaves, the spent-journal prune must not
	pruneHeightBucketBelow(txn, storage.CommitBucketName, cutoff, true)
	pruneHeightBucketBelow(txn, storage.CommitSpentBucketName, cutoff, false)
}

// pruneHeightBucketBelow deletes every entry of a height-major bucket whose
// leading heightLE(8) is below cutoff. inTrie tells whether this bucket backs
// the state commitment (the mempool:commit bucket does, the spent journal does
// NOT), so the trie leaf is dropped for exactly the committed bucket.
func pruneHeightBucketBelow(txn *bbolt.Tx, name []byte, cutoff uint64, inTrie bool) {
	bucket := txn.Bucket(name)
	if bucket == nil {
		return
	}

	var keys [][]byte
	c := bucket.Cursor()
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		if len(k) < 8 {
			continue
		}
		if utils.UnpackUint64LE(k[:8]) < cutoff {
			keys = append(keys, append([]byte{}, k...))
		}
	}
	for _, k := range keys {
		_ = bucket.Delete(k)
		if inTrie {
			trieSetCommit(txn, k, nil)
		}
	}
}
