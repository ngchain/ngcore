package ngstate

import (
	"bytes"
	"math/big"
	"math/rand"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/statetrie"
)

// assertTrieConsistent checks the incremental root the choke points maintained
// equals a from-scratch RebuildTrie over the current plain buckets. RebuildTrie
// rebuilds the SAME state:trie bucket in place; since the live entry SET is
// identical either way, the roots must match AND the store is left canonical,
// so incremental updates after the check stay valid.
func assertTrieConsistent(t *testing.T, txn *bbolt.Tx, step string) {
	t.Helper()
	inc := append([]byte{}, StateRoot(txn)...)
	if err := RebuildTrie(txn); err != nil {
		t.Fatalf("%s: rebuild: %v", step, err)
	}
	rebuilt := StateRoot(txn)
	if !bytes.Equal(inc, rebuilt) {
		t.Fatalf("%s: incremental root %x != rebuilt root %x", step, inc, rebuilt)
	}
}

// TestStateTrieInvariant drives a fixed-seed randomized sequence of
// balance/contract/key/commit mutations plus per-height block unwinds through
// the REAL trie-maintenance choke points, asserting the incrementally kept
// StateRoot equals a from-scratch RebuildTrie root at every step. A missed
// choke point (a bucket write that skips the trie) diverges the two roots.
func TestStateTrieInvariant(t *testing.T) {
	db := newTestDB(t)

	// archive on: the unwind path reads recorded changesets, and it also
	// reverts addr:bal/addr:contract/addr:key directly — those reverts must
	// keep the trie in sync, which is exactly what this test pins
	state := newTestState(t, db)
	state.Archive = true

	rng := rand.New(rand.NewSource(0xC0FFEE))

	// a small pool of addresses so writes collide (overwrite/delete paths)
	addrs := make([]ngtypes.Address, 8)
	for i := range addrs {
		addrs[i] = testAddr(byte(0x10 + i))
	}
	// pre-generated keypairs so registerPubKey has real full-envelope txs
	privs := make([]*ngtypes.PrivateKey, 4)
	keyAddrs := make([]ngtypes.Address, 4)
	for i := range privs {
		p, err := ngtypes.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		privs[i] = p
		keyAddrs[i] = ngtypes.NewAddress(p)
	}

	err := db.Update(func(txn *bbolt.Tx) error {
		height := uint64(0)
		appliedHeights := []uint64{} // heights with recorded changesets, for unwind

		mutateAtHeight := func(h uint64) {
			state.cs = newChangeset(h)
			defer func() { state.cs = nil }()

			ops := 3 + rng.Intn(5)
			for i := 0; i < ops; i++ {
				switch rng.Intn(5) {
				case 0: // balance write (zero sometimes -> delete leaf)
					addr := addrs[rng.Intn(len(addrs))]
					amt := big.NewInt(int64(rng.Intn(5))) // 0..4, 0 == absent
					if err := setBalance(txn, state.cs, addr, amt); err != nil {
						t.Fatal(err)
					}
				case 1: // contract deploy/overwrite via the real setContract
					addr := addrs[rng.Intn(len(addrs))]
					src := []byte{byte(rng.Intn(256)), byte(i)}
					c := ngtypes.NewContract(addr, src, nil)
					if err := setContract(txn, state.cs, c); err != nil {
						t.Fatal(err)
					}
				case 2: // contract destroy
					addr := addrs[rng.Intn(len(addrs))]
					if err := delContract(txn, state.cs, addr); err != nil {
						t.Fatal(err)
					}
				case 3: // key registry reveal via a real signed full-envelope tx
					idx := rng.Intn(len(privs))
					tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, h,
						testAddr(0x01), big.NewInt(1), big.NewInt(1), nil, nil)
					tx.Salt = []byte("0123456789abcdef0123456789abcdef")
					if err := tx.Signature(privs[idx]); err != nil {
						t.Fatal(err)
					}
					if err := registerPubKey(txn, state.cs, tx); err != nil {
						t.Fatal(err)
					}
				case 4: // pending commitment record/consume
					addr := addrs[rng.Intn(len(addrs))]
					hash := make([]byte, ngtypes.HashSize)
					rng.Read(hash)
					if err := putCommit(txn, h, hash, addr); err != nil {
						t.Fatal(err)
					}
					if rng.Intn(2) == 0 {
						if err := consumeCommit(txn, h, hash); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
		}

		// forward-apply a run of heights, checking the invariant after each
		for r := 0; r < 30; r++ {
			height++
			mutateAtHeight(height)
			appliedHeights = append(appliedHeights, height)
			assertTrieConsistent(t, txn, "apply")

			// occasionally unwind the most recent height through the real
			// unwindHeightTxn (reverts bal/contract/key + drops/restores commits)
			if len(appliedHeights) > 3 && rng.Intn(3) == 0 {
				top := appliedHeights[len(appliedHeights)-1]
				unwindHeightTxn(txn, top)
				appliedHeights = appliedHeights[:len(appliedHeights)-1]
				height--
				assertTrieConsistent(t, txn, "unwind")
			}
		}

		// silence the unused keyAddrs (kept for readability of intent)
		_ = keyAddrs
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestStateProofRoundTrip proves a leaf and verifies it, and checks a tampered
// proof fails, using the same statetrie the RPC serves.
func TestStateProofRoundTrip(t *testing.T) {
	db := newTestDB(t)

	addr := testAddr(0x22)
	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(42)); err != nil {
			return err
		}
		root, path, value, valueHash, proof, err := StateProof(txn, "balance", addr[:])
		if err != nil {
			return err
		}
		if !bytes.Equal(value, big.NewInt(42).Bytes()) {
			t.Fatalf("proof value %x != 42-bytes", value)
		}
		if !statetrie.Verify(root, path, valueHash, proof) {
			t.Fatal("valid proof rejected")
		}
		// tamper: flip a sibling
		proof[0] = append([]byte{}, proof[0]...)
		proof[0][0] ^= 0xff
		if statetrie.Verify(root, path, valueHash, proof) {
			t.Fatal("tampered proof accepted")
		}

		// absence proof for an untouched address
		other := testAddr(0x99)
		r2, p2, v2, vh2, pr2, err := StateProof(txn, "balance", other[:])
		if err != nil {
			return err
		}
		if len(v2) != 0 {
			t.Fatalf("expected absent value, got %x", v2)
		}
		if !statetrie.Verify(r2, p2, vh2, pr2) {
			t.Fatal("absence proof rejected")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
