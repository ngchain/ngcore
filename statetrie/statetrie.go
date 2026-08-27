// Package statetrie implements the consensus state commitment: a
// 256-level sparse binary Merkle tree (SMT-256) hashed with BLAKE3-256
// (utils.Hash256) throughout, so the whole construction stays
// post-quantum (hash-based, no pairings/KZG).
//
// Layout
//
//   - Every consensus entry lives at a fixed leaf position
//     path = Hash256(domainTag ‖ rawKey), 32 bytes = 256 bits, walked
//     MSB-first from the root.
//   - Leaf hash:      Hash256(0x00 ‖ path ‖ valueHash) where
//     valueHash = Hash256(encodedValue); an ABSENT leaf is zeros32.
//   - Internal node:  Hash256(0x01 ‖ left ‖ right).
//   - Empty-subtree defaults: default[0] = zeros32 (empty leaf),
//     default[h+1] = Hash256(0x01 ‖ default[h] ‖ default[h]) — an
//     internal node over two empty children IS the empty default, so no
//     special-casing is needed when hashing upward.
//
// Only non-default nodes are stored, keyed by depth(2B BE) ‖ path with
// the bits below that depth zeroed. Deleting an entry (valueHash ==
// zeros32) removes the stored nodes back to the defaults, so the same
// SET of entries always yields the same stored nodes and the same root,
// independent of operation order. Determinism is consensus-critical:
// nothing here iterates a map into a hash, reads a clock, or draws
// randomness.
package statetrie

import (
	"bytes"
	"encoding/binary"

	"github.com/ngchain/ngcore/utils"
)

const (
	// HashSize is the byte length of every node hash and path
	HashSize = 32
	// Depth is the number of tree levels below the root (one per path bit)
	Depth = 256
)

// Domain tags: the single byte prefixed to the raw key before hashing
// into a leaf path, separating the consensus state domains
const (
	// DomainBalance commits addr -> balance (minimal big-endian bytes;
	// a zero balance is an ABSENT leaf)
	DomainBalance byte = 0x01
	// DomainKey commits the append-only pubkey registry:
	// addr -> scheme ‖ pubkey
	DomainKey byte = 0x02
	// DomainContract commits addr -> rlp(storedContract) exactly as the
	// contract bucket stores it (code is referenced via its CodeHash, so
	// code dedup does not affect the commitment)
	DomainContract byte = 0x03
	// DomainCommit commits the pending private-mempool commitments:
	// LE₈(height)‖Hash -> From (the mempool:commit bucket key/value)
	DomainCommit byte = 0x04
)

// node-hash domain separation
const (
	leafTag     byte = 0x00
	internalTag byte = 0x01
)

// defaults[h] is the hash of an EMPTY subtree of height h (h=0 a leaf,
// h=Depth the empty root), precomputed once at package init
var defaults [Depth + 1][HashSize]byte

func init() {
	// defaults[0] stays zeros32
	buf := make([]byte, 1+2*HashSize)
	buf[0] = internalTag
	for h := 0; h < Depth; h++ {
		copy(buf[1:], defaults[h][:])
		copy(buf[1+HashSize:], defaults[h][:])
		copy(defaults[h+1][:], utils.Hash256(buf))
	}
}

// EmptyRoot returns the root of the empty tree
func EmptyRoot() []byte {
	root := make([]byte, HashSize)
	copy(root, defaults[Depth][:])
	return root
}

// NodeStore is the persistence interface of the tree: only non-default
// nodes are kept. Get returns nil when the key is absent.
type NodeStore interface {
	Get(key []byte) []byte
	Put(key, val []byte) error
	Delete(key []byte) error
}

// MemStore is the in-memory NodeStore (genesis computation and tests)
type MemStore struct {
	m map[string][]byte
}

func NewMemStore() *MemStore {
	return &MemStore{m: make(map[string][]byte)}
}

func (s *MemStore) Get(key []byte) []byte {
	return s.m[string(key)]
}

func (s *MemStore) Put(key, val []byte) error {
	s.m[string(key)] = append([]byte{}, val...)
	return nil
}

func (s *MemStore) Delete(key []byte) error {
	delete(s.m, string(key))
	return nil
}

// Len reports how many non-default nodes the store holds
func (s *MemStore) Len() int { return len(s.m) }

// LeafPath maps a domain-tagged raw key to its fixed leaf position
func LeafPath(domain byte, rawKey []byte) []byte {
	buf := make([]byte, 1+len(rawKey))
	buf[0] = domain
	copy(buf[1:], rawKey)
	return utils.Hash256(buf)
}

// ValueHash hashes an encoded leaf value
func ValueHash(encodedValue []byte) []byte {
	return utils.Hash256(encodedValue)
}

// ZeroHash returns the all-zero valueHash that Update treats as delete
func ZeroHash() []byte {
	return make([]byte, HashSize)
}

// leafHash computes the leaf node hash: zeros32 when absent
func leafHash(path, valueHash []byte) []byte {
	if isZero(valueHash) {
		return make([]byte, HashSize)
	}
	buf := make([]byte, 1+2*HashSize)
	buf[0] = leafTag
	copy(buf[1:], path)
	copy(buf[1+HashSize:], valueHash)
	return utils.Hash256(buf)
}

func internalHash(left, right []byte) []byte {
	buf := make([]byte, 1+2*HashSize)
	buf[0] = internalTag
	copy(buf[1:], left)
	copy(buf[1+HashSize:], right)
	return utils.Hash256(buf)
}

func isZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// pathBit returns bit i of the path, MSB-first (bit 0 = the root's
// branching decision)
func pathBit(path []byte, i int) byte {
	return (path[i/8] >> (7 - i%8)) & 1
}

// nodeKey builds the storage key of the node at the given depth on the
// path: depth(2B BE) ‖ path with the bits below depth zeroed
func nodeKey(depth int, path []byte) []byte {
	key := make([]byte, 2+HashSize)
	binary.BigEndian.PutUint16(key, uint16(depth))

	full := depth / 8
	copy(key[2:2+full], path[:full])
	if rem := depth % 8; rem != 0 {
		key[2+full] = path[full] & (byte(0xff) << (8 - rem))
	}
	return key
}

// siblingKey is nodeKey of the sibling at depth: the same prefix with
// bit depth-1 flipped
func siblingKey(depth int, path []byte) []byte {
	key := nodeKey(depth, path)
	i := depth - 1
	key[2+i/8] ^= 1 << (7 - i%8)
	return key
}

// getNode reads the node hash at a store key, falling back to the
// empty default of that height
func getNode(store NodeStore, key []byte, height int) []byte {
	if v := store.Get(key); v != nil {
		return v
	}
	return defaults[height][:]
}

// setNode writes the node hash, deleting the entry when it equals the
// empty default of its height (keeps the store canonical: same entry
// set -> same stored nodes)
func setNode(store NodeStore, key []byte, height int, val []byte) error {
	if bytes.Equal(val, defaults[height][:]) {
		return store.Delete(key)
	}
	return store.Put(key, val)
}

// Update sets the leaf at path to valueHash (zeros32 = delete) and
// rehashes the branch up to the root
func Update(store NodeStore, path, valueHash []byte) error {
	cur := leafHash(path, valueHash)
	if err := setNode(store, nodeKey(Depth, path), 0, cur); err != nil {
		return err
	}

	for depth := Depth; depth >= 1; depth-- {
		height := Depth - depth // height of the two children
		sib := getNode(store, siblingKey(depth, path), height)

		if pathBit(path, depth-1) == 0 {
			cur = internalHash(cur, sib)
		} else {
			cur = internalHash(sib, cur)
		}

		if err := setNode(store, nodeKey(depth-1, path), height+1, cur); err != nil {
			return err
		}
	}

	return nil
}

// Root returns the current tree root (the empty-tree default when
// nothing is stored)
func Root(store NodeStore) []byte {
	root := make([]byte, HashSize)
	copy(root, getNode(store, nodeKey(0, make([]byte, HashSize)), Depth))
	return root
}

// Prove returns the Merkle branch of the leaf at path: exactly 256
// sibling hashes ordered bottom-up (proof[0] is the leaf's direct
// sibling, proof[255] the root's other child). Uncompacted on purpose —
// simple and self-describing; callers may hex/omit as they wish.
func Prove(store NodeStore, path []byte) [][]byte {
	proof := make([][]byte, Depth)
	for depth := Depth; depth >= 1; depth-- {
		height := Depth - depth
		sib := make([]byte, HashSize)
		copy(sib, getNode(store, siblingKey(depth, path), height))
		proof[height] = sib
	}
	return proof
}

// Verify checks a Merkle branch: does the leaf (path, valueHash) fold
// through the 256 siblings into root? valueHash == zeros32 verifies a
// proof of ABSENCE.
func Verify(root, path, valueHash []byte, proof [][]byte) bool {
	if len(root) != HashSize || len(path) != HashSize || len(valueHash) != HashSize || len(proof) != Depth {
		return false
	}

	cur := leafHash(path, valueHash)
	for height := 0; height < Depth; height++ {
		sib := proof[height]
		if len(sib) != HashSize {
			return false
		}
		depth := Depth - height
		if pathBit(path, depth-1) == 0 {
			cur = internalHash(cur, sib)
		} else {
			cur = internalHash(sib, cur)
		}
	}

	return bytes.Equal(cur, root)
}
