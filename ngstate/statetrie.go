package ngstate

import (
	"bytes"
	"math/big"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/statetrie"
	"github.com/ngchain/ngcore/storage"
)

// The consensus state commitment is a BLAKE3 SMT-256 (statetrie) kept in
// lock-step with the four committed state buckets:
//
//	addr:bal        -> DomainBalance  leaf: Hash256(balance minimal BE bytes)
//	addr:key        -> DomainKey      leaf: Hash256(scheme‖pubkey entry)
//	addr:contract   -> DomainContract leaf: Hash256(rlp(storedContract))
//	mempool:commit  -> DomainCommit   leaf: Hash256(From bytes), keyed by the
//	                                   full heightLE(8)‖Hash(32) bucket key
//
// A zero balance / a nil value is an ABSENT leaf (statetrie deletes it), so
// the SAME set of live entries always yields the same root regardless of the
// order the writes arrived — which is what lets the incremental root track a
// from-scratch rebuild. Every state write choke point calls the matching
// trieSet* helper so the tree never drifts from the buckets.

// bboltNodeStore adapts the state:trie bucket of a write txn to
// statetrie.NodeStore. All trie mutations run inside the block-apply txn, so
// a failed apply rolls back the trie with the rest of the state.
type bboltNodeStore struct {
	bucket *bbolt.Bucket
}

func newNodeStore(txn *bbolt.Tx) *bboltNodeStore {
	return &bboltNodeStore{bucket: txn.Bucket(storage.StateTrieBucketName)}
}

func (s *bboltNodeStore) Get(key []byte) []byte {
	v := s.bucket.Get(key)
	if v == nil {
		return nil
	}
	// bbolt values are only valid for the life of the txn and must not be
	// mutated; statetrie hashes them read-only, but copy to be safe against
	// any future caller that retains the slice
	return append([]byte{}, v...)
}

func (s *bboltNodeStore) Put(key, val []byte) error { return s.bucket.Put(key, val) }

func (s *bboltNodeStore) Delete(key []byte) error { return s.bucket.Delete(key) }

// --- per-domain leaf updates: called from the state write choke points ---

// trieSetLeaf mirrors one state write into the commitment: it sets the leaf at
// (domain, key) to ValueHash(value), or DELETES it (ZeroHash) when value is
// empty. It single-sources the "empty ⇒ absent leaf" rule every domain relies
// on — the same rule StateProof and RebuildTrie encode — so the tree can never
// drift between the incremental and from-scratch roots. All the typed trieSet*
// wrappers below funnel through it.
func trieSetLeaf(txn *bbolt.Tx, domain byte, key, value []byte) {
	path := statetrie.LeafPath(domain, key)
	var vh []byte
	if len(value) == 0 {
		vh = statetrie.ZeroHash()
	} else {
		vh = statetrie.ValueHash(value)
	}
	_ = statetrie.Update(newNodeStore(txn), path, vh)
}

// trieSetBalance mirrors an addr:bal write into the trie. A zero balance is
// an absent leaf (statetrie deletes it), matching "balance 0 == absent".
func trieSetBalance(txn *bbolt.Tx, addr ngtypes.Address, balance *big.Int) {
	var val []byte
	if balance != nil && balance.Sign() != 0 {
		val = balance.Bytes()
	}
	trieSetLeaf(txn, statetrie.DomainBalance, addr[:], val)
}

// trieSetKey mirrors an addr:key write. entry is scheme‖pubkey; nil deletes
// (a reorg dropping a first-reveal). The registry is append-only in normal
// operation, so this is only ever an insert or an unwind-delete.
func trieSetKey(txn *bbolt.Tx, addr ngtypes.Address, entry []byte) {
	trieSetLeaf(txn, statetrie.DomainKey, addr[:], entry)
}

// trieSetContract mirrors an addr:contract write. storedRLP is the exact
// rlp(storedContract) blob the bucket holds (code referenced by hash, so the
// code-dedup bucket never touches the commitment); nil deletes the slot.
func trieSetContract(txn *bbolt.Tx, addr ngtypes.Address, storedRLP []byte) {
	trieSetLeaf(txn, statetrie.DomainContract, addr[:], storedRLP)
}

// trieSetCommit mirrors a mempool:commit write. The raw key is the full
// bucket key heightLE(8)‖Hash(32); from is the committer address, nil deletes
// (a reveal consuming it, a height-undo, or a prune).
func trieSetCommit(txn *bbolt.Tx, bucketKey []byte, from []byte) {
	trieSetLeaf(txn, statetrie.DomainCommit, bucketKey, from)
}

// trieSetBeacon mirrors the single native-randomness-beacon leaf into the trie
// (statetrie DomainBeacon at the fixed BeaconStateKey). A nil seed deletes it —
// a reorg unwinding the first post-genesis block back to genesis.
func trieSetBeacon(txn *bbolt.Tx, seed []byte) {
	trieSetLeaf(txn, statetrie.DomainBeacon, ngtypes.BeaconStateKey, seed)
}

// StateRoot returns the current consensus-state commitment root of the txn.
func StateRoot(txn *bbolt.Tx) []byte {
	return statetrie.Root(newNodeStore(txn))
}

// RebuildTrie clears the state:trie bucket and re-inserts every live entry of
// the four committed buckets from scratch, so its Root is derived purely from
// the current plain state. Used after a bulk state reset (sheet/replay) and by
// the invariant test to check the incremental root against a fresh build.
func RebuildTrie(txn *bbolt.Tx) error {
	if err := txn.DeleteBucket(storage.StateTrieBucketName); err != nil {
		return err
	}
	if _, err := txn.CreateBucket(storage.StateTrieBucketName); err != nil {
		return err
	}

	store := newNodeStore(txn)

	// balances
	if b := txn.Bucket(storage.Addr2BalBucketName); b != nil {
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			bal := new(big.Int).SetBytes(v)
			if bal.Sign() == 0 {
				continue // a zero balance is an absent leaf
			}
			var addr ngtypes.Address
			copy(addr[:], k)
			path := statetrie.LeafPath(statetrie.DomainBalance, addr[:])
			if err := statetrie.Update(store, path, statetrie.ValueHash(bal.Bytes())); err != nil {
				return err
			}
		}
	}

	// key registry
	if b := txn.Bucket(storage.KeyRegistryBucketName); b != nil {
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var addr ngtypes.Address
			copy(addr[:], k)
			path := statetrie.LeafPath(statetrie.DomainKey, addr[:])
			if err := statetrie.Update(store, path, statetrie.ValueHash(v)); err != nil {
				return err
			}
		}
	}

	// contract slots
	if b := txn.Bucket(storage.ContractBucketName); b != nil {
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var addr ngtypes.Address
			copy(addr[:], k)
			path := statetrie.LeafPath(statetrie.DomainContract, addr[:])
			if err := statetrie.Update(store, path, statetrie.ValueHash(v)); err != nil {
				return err
			}
		}
	}

	// pending commitments
	if b := txn.Bucket(storage.CommitBucketName); b != nil {
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			path := statetrie.LeafPath(statetrie.DomainCommit, k)
			if err := statetrie.Update(store, path, statetrie.ValueHash(v)); err != nil {
				return err
			}
		}
	}

	// native randomness beacon (single leaf)
	if b := txn.Bucket(storage.BeaconBucketName); b != nil {
		if v := b.Get(ngtypes.BeaconStateKey); v != nil {
			path := statetrie.LeafPath(statetrie.DomainBeacon, ngtypes.BeaconStateKey)
			if err := statetrie.Update(store, path, statetrie.ValueHash(v)); err != nil {
				return err
			}
		}
	}

	return nil
}

// CheckStateRoot compares the txn's incremental root against a header's
// committed StateRoot, returning ErrBlockStateRootInvalid on a mismatch.
// Called after State.Upgrade on both the fast path and the reorg replay; a
// mismatch inside the write txn rolls the apply back.
func CheckStateRoot(txn *bbolt.Tx, want []byte) error {
	got := StateRoot(txn)
	if !bytes.Equal(got, want) {
		return errors.Wrapf(ngtypes.ErrBlockStateRootInvalid,
			"post-state root %x does not match the committed %x", got, want)
	}
	return nil
}

// DryApplyRoot applies a block's Upgrade in a throwaway write txn and ROLLS
// BACK, returning the resulting post-state root. Mining uses it to seal the
// StateRoot into the header BEFORE pow: the root depends on the block's own
// contents (incl. the miner's generate), so it must be known pre-seal.
func DryApplyRoot(state *State, block *ngtypes.FullBlock) (root []byte, err error) {
	// a manual write txn we always roll back: the block is not yet valid to
	// commit (no pow), we only want the root it would produce
	txn, err := state.DB.Begin(true)
	if err != nil {
		return nil, err
	}
	defer func() {
		// Rollback never returns a useful error here; the txn is discarded
		_ = txn.Rollback()
	}()

	if err := state.Upgrade(txn, block); err != nil {
		return nil, err
	}

	root = append([]byte{}, StateRoot(txn)...)
	return root, nil
}

var (
	// ErrUnknownStateDomain is returned by StateProof for a domain string
	// outside {balance, key, contract, commit}
	ErrUnknownStateDomain = errors.New("unknown state domain")
)

// stateDomain maps a domain name to its statetrie tag and the bucket that
// holds the raw leaf value, so a proof can read the committed value.
func stateDomain(domain string) (tag byte, bucket []byte, err error) {
	switch domain {
	case "balance":
		return statetrie.DomainBalance, storage.Addr2BalBucketName, nil
	case "key":
		return statetrie.DomainKey, storage.KeyRegistryBucketName, nil
	case "contract":
		return statetrie.DomainContract, storage.ContractBucketName, nil
	case "commit":
		return statetrie.DomainCommit, storage.CommitBucketName, nil
	case "beacon":
		// single leaf; the caller passes ngtypes.BeaconStateKey as rawKey
		return statetrie.DomainBeacon, storage.BeaconBucketName, nil
	default:
		return 0, nil, errors.Wrapf(ErrUnknownStateDomain, "%q", domain)
	}
}

// StateProof returns a self-contained inclusion/absence proof of one leaf: the
// committed root, the leaf path, the value bytes stored in the domain's bucket
// (nil when absent), the leaf valueHash, and the 256-sibling Merkle branch.
// statetrie.Verify(root, path, valueHash, proof) accepts it; a zero valueHash
// proves absence. rawKey is the domain's bucket key (address bytes for
// balance/key/contract, heightLE(8)‖Hash(32) for commit).
func StateProof(txn *bbolt.Tx, domain string, rawKey []byte) (root, path, value, valueHash []byte, proof [][]byte, err error) {
	tag, bucketName, err := stateDomain(domain)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	path = statetrie.LeafPath(tag, rawKey)

	if b := txn.Bucket(bucketName); b != nil {
		if v := b.Get(rawKey); v != nil {
			value = append([]byte{}, v...)
		}
	}

	// the leaf valueHash mirrors the trie's per-domain encoding: a zero
	// balance (empty stored bytes) is an absent leaf, matching trieSetBalance
	if len(value) == 0 {
		valueHash = statetrie.ZeroHash()
	} else {
		valueHash = statetrie.ValueHash(value)
	}

	store := newNodeStore(txn)
	root = append([]byte{}, statetrie.Root(store)...)
	proof = statetrie.Prove(store, path)
	return root, path, value, valueHash, proof, nil
}
