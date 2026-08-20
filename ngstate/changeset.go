package ngstate

import (
	"bytes"
	"math/big"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// changeset is the per-block pre-image recorder. It is set on the State
// for the duration of one Upgrade (single-threaded inside the write txn)
// and consulted by the write helpers BEFORE they overwrite a value, so
// each mutated address's prior value is captured under the block height.
// A nil recorder (the default, and always on non-archive nodes or on the
// genesis/snapshot/lazy-fork seeding paths) records nothing.
type changeset struct {
	height uint64
}

func newChangeset(height uint64) *changeset { return &changeset{height: height} }

// value tags: an address either had NO prior value at this domain
// (tombstone) or a concrete one. Balance 0 and "absent" are semantically
// identical, but contracts and the reorg unwind need the distinction, so
// every pre-image carries a one-byte tag.
const (
	preimageAbsent  = 0x00
	preimagePresent = 0x01
)

func taggedPreimage(old []byte) []byte {
	if old == nil {
		return []byte{preimageAbsent}
	}
	out := make([]byte, 1+len(old))
	out[0] = preimagePresent
	copy(out[1:], old)
	return out
}

// splitPreimage returns (value, present); value is nil when absent
func splitPreimage(tagged []byte) ([]byte, bool) {
	if len(tagged) == 0 || tagged[0] == preimageAbsent {
		return nil, false
	}
	return tagged[1:], true
}

// csKey is block-major: heightLE ‖ addr, so a cursor range over one
// height yields every address changed at that height (reorg unwind)
func csKey(height uint64, addr ngtypes.Address) []byte {
	key := make([]byte, 8+ngtypes.AddressSize)
	copy(key[:8], utils.PackUint64LE(height))
	copy(key[8:], addr[:])
	return key
}

// histKey is addr-major: addr ‖ heightLE, so a prefix cursor yields an
// address's change-heights in ascending order (point queries)
func histKey(addr ngtypes.Address, height uint64) []byte {
	key := make([]byte, ngtypes.AddressSize+8)
	copy(key[:ngtypes.AddressSize], addr[:])
	copy(key[ngtypes.AddressSize:], utils.PackUint64LE(height))
	return key
}

// recordBal captures addr's current balance as the pre-image at this
// height, once (the first write in a block wins: that is the value that
// held at height-1). No-op on a nil recorder.
func (cs *changeset) recordBal(txn *bbolt.Tx, addr ngtypes.Address) {
	if cs == nil {
		return
	}
	csb := txn.Bucket(storage.BalChangeSetBucketName)
	k := csKey(cs.height, addr)
	if csb.Get(k) != nil {
		return // already captured this height
	}
	old := txn.Bucket(storage.Addr2BalBucketName).Get(addr[:])
	_ = csb.Put(k, taggedPreimage(old))
	_ = txn.Bucket(storage.BalHistBucketName).Put(histKey(addr, cs.height), nil)
}

// recordContract captures addr's current contract-slot blob as the
// pre-image at this height, once. No-op on a nil recorder.
func (cs *changeset) recordContract(txn *bbolt.Tx, addr ngtypes.Address) {
	if cs == nil {
		return
	}
	csb := txn.Bucket(storage.ContractChangeSetBucketName)
	k := csKey(cs.height, addr)
	if csb.Get(k) != nil {
		return
	}
	old := txn.Bucket(storage.ContractBucketName).Get(addr[:])
	_ = csb.Put(k, taggedPreimage(old))
	_ = txn.Bucket(storage.ContractHistBucketName).Put(histKey(addr, cs.height), nil)
}

// recordKey captures a key-registry reveal at this height. The registry
// is append-only, so the pre-image is always absent; recording it lets a
// reorg drop the reveal. No index (no historical key query). No-op on nil.
func (cs *changeset) recordKey(txn *bbolt.Tx, addr ngtypes.Address) {
	if cs == nil {
		return
	}
	csb := txn.Bucket(storage.KeyChangeSetBucketName)
	k := csKey(cs.height, addr)
	if csb.Get(k) != nil {
		return
	}
	_ = csb.Put(k, []byte{preimageAbsent})
}

// firstChangeHeightAfter finds the smallest height > target at which addr
// changed, using the addr-major history index. The pre-image recorded at
// that height is the value that held AT target. found=false means addr
// never changed after target, so the current plain state is the answer.
func firstChangeHeightAfter(txn *bbolt.Tx, histBucket []byte, addr ngtypes.Address, target uint64) (uint64, bool) {
	c := txn.Bucket(histBucket).Cursor()
	seek := histKey(addr, target+1)
	k, _ := c.Seek(seek)
	if k == nil || len(k) != ngtypes.AddressSize+8 {
		return 0, false
	}
	// the seek may land on the next address; verify the prefix
	var got ngtypes.Address
	copy(got[:], k[:ngtypes.AddressSize])
	if got != addr {
		return 0, false
	}
	return utils.UnpackUint64LE(k[ngtypes.AddressSize:]), true
}

// balanceAtHeight resolves addr's balance as of the given height: the
// pre-image at the first change after height, else the current balance
func balanceAtHeight(txn *bbolt.Tx, addr ngtypes.Address, height uint64) *big.Int {
	m, ok := firstChangeHeightAfter(txn, storage.BalHistBucketName, addr, height)
	if !ok {
		return getBalance(txn, addr)
	}
	tagged := txn.Bucket(storage.BalChangeSetBucketName).Get(csKey(m, addr))
	val, present := splitPreimage(tagged)
	if !present {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(val)
}

// contractAtHeight resolves addr's contract slot as of the given height:
// the pre-image at the first change after height, else the current slot.
// ok=false when the address had no slot at that height.
func contractAtHeight(txn *bbolt.Tx, addr ngtypes.Address, height uint64) (*ngtypes.Contract, bool, error) {
	m, changed := firstChangeHeightAfter(txn, storage.ContractHistBucketName, addr, height)
	if !changed {
		acc, err := getContract(txn, addr)
		if err != nil {
			return nil, false, nil // no slot now and none recorded after: absent
		}
		return acc, true, nil
	}
	tagged := txn.Bucket(storage.ContractChangeSetBucketName).Get(csKey(m, addr))
	raw, present := splitPreimage(tagged)
	if !present {
		return nil, false, nil // slot did not exist at that height
	}
	acc, err := decodeStoredContract(txn, raw)
	if err != nil {
		return nil, false, err
	}
	return acc, true, nil
}

// --- reorg unwind: revert a height's writes from its recorded pre-images ---

// changesetCovers reports whether the changeset store reaches down to
// height h. Every applied archive height has at least the coinbase
// balance change, so the presence of h implies the whole range above it
// is covered too; absence means the node cannot unwind that far (a
// snapshot-started node below its origin) and must fall back to replay
func changesetCovers(txn *bbolt.Tx, h uint64) bool {
	c := txn.Bucket(storage.BalChangeSetBucketName).Cursor()
	prefix := utils.PackUint64LE(h)
	k, _ := c.Seek(prefix)
	return k != nil && bytes.HasPrefix(k, prefix)
}

// revertDomain restores every address mutated at height h in one state
// domain to its pre-image, then drops that height's changeset and index
// entries. present -> Put the old value, absent -> Delete
func revertDomain(txn *bbolt.Tx, csName, stateName, histName []byte, h uint64) {
	csb := txn.Bucket(csName)
	stateB := txn.Bucket(stateName)
	prefix := utils.PackUint64LE(h)

	type entry struct {
		addr   ngtypes.Address
		tagged []byte
	}
	var entries []entry

	c := csb.Cursor()
	for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
		if len(k) != 8+ngtypes.AddressSize {
			continue
		}
		var addr ngtypes.Address
		copy(addr[:], k[8:])
		entries = append(entries, entry{addr, append([]byte{}, v...)})
	}

	for _, e := range entries {
		if val, present := splitPreimage(e.tagged); present {
			_ = stateB.Put(e.addr[:], val)
		} else {
			_ = stateB.Delete(e.addr[:])
		}
		if histName != nil {
			_ = txn.Bucket(histName).Delete(histKey(e.addr, h))
		}
		_ = csb.Delete(csKey(h, e.addr))
	}
}

// unwindHeightTxn reverts every state change applied at height h
func unwindHeightTxn(txn *bbolt.Tx, h uint64) {
	revertDomain(txn, storage.BalChangeSetBucketName, storage.Addr2BalBucketName, storage.BalHistBucketName, h)
	revertDomain(txn, storage.ContractChangeSetBucketName, storage.ContractBucketName, storage.ContractHistBucketName, h)
	// the key registry is append-only: a recorded reveal means the entry
	// did not exist before, so unwinding just drops it (no history index)
	revertDomain(txn, storage.KeyChangeSetBucketName, storage.KeyRegistryBucketName, nil, h)
}
