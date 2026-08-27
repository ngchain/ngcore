package ngstate

import (
	"bytes"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/statetrie"
	"github.com/ngchain/ngcore/storage"
)

// beaconBlock builds a minimal block carrying just the fields updateBeacon reads:
// network, height, parent hash, and each tx's reveal salt.
func beaconBlock(height uint64, prevHash []byte, salts ...string) *ngtypes.FullBlock {
	txs := make([]*ngtypes.FullTx, 0, len(salts))
	for _, s := range salts {
		txs = append(txs, &ngtypes.FullTx{Salt: []byte(s)})
	}
	return &ngtypes.FullBlock{
		BlockHeader: &ngtypes.BlockHeader{
			Network:       ngtypes.ZERONET,
			Height:        height,
			PrevBlockHash: prevHash,
		},
		Txs: txs,
	}
}

func zeros32() []byte { return make([]byte, ngtypes.HashSize) }

// randWat is a contract whose ng:main reads the beacon seed via crypto.random
// (32 bytes into memory) and stores it under key "r", so a test can assert the
// exact seed reached the contract.
const randWat = `
(module
  (import "crypto" "random" (func $rand (param i32) (result i32)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "r")
  (func (export "ng:main")
    (drop (call $rand (i32.const 8)))
    (drop (call $set (i32.const 0) (i32.const 1) (i32.const 8) (i32.const 32)))))
`

// TestContractReadsBeacon: a contract calling crypto.random receives the exact
// beacon seed currently in state (the parent block's finalized seed).
func TestContractReadsBeacon(t *testing.T) {
	db := newTestDB(t)
	addr := testAddr(0x5a)

	seed := make([]byte, ngtypes.HashSize)
	for i := range seed {
		seed[i] = byte(0xA0 + i)
	}

	if err := db.Update(func(txn *bbolt.Tx) error {
		// seed the beacon as if a prior block finalized it
		if err := txn.Bucket(storage.BeaconBucketName).Put(ngtypes.BeaconStateKey, seed); err != nil {
			return err
		}

		acc := ngtypes.NewContract(addr, mustWat(randWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, rentFund) // fund the kv storage deposit

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		reloaded, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if got := reloaded.Context.Get("r"); !bytes.Equal(got, seed) {
			t.Fatalf("contract read beacon %x, want seeded %x", got, seed)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestBeaconEmptyIsZero: on a fresh state the beacon reads as 32 zero bytes.
func TestBeaconEmptyIsZero(t *testing.T) {
	db := newTestDB(t)
	if err := db.View(func(txn *bbolt.Tx) error {
		if got := getBeacon(txn); !bytes.Equal(got, zeros32()) {
			t.Fatalf("empty beacon = %x, want 32 zero bytes", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestBeaconGenesisSkipped: updateBeacon is a no-op at height 0, so the genesis
// post-state root stays beacon-free (matching ngtypes.genesisStateRoot).
func TestBeaconGenesisSkipped(t *testing.T) {
	db := newTestDB(t)
	if err := db.Update(func(txn *bbolt.Tx) error {
		before := append([]byte{}, StateRoot(txn)...)
		updateBeacon(txn, nil, beaconBlock(0, zeros32(), "0123456789abcdef"))
		if got := getBeacon(txn); !bytes.Equal(got, zeros32()) {
			t.Fatalf("genesis beacon = %x, want unchanged zeros", got)
		}
		if after := StateRoot(txn); !bytes.Equal(before, after) {
			t.Fatalf("genesis StateRoot changed by beacon: %x -> %x", before, after)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestBeaconUpdateDeterministicAndCommitted: a post-genesis block advances the
// beacon to a non-zero seed, the seed is a deterministic function of the block,
// it is committed under StateRoot (a verifiable DomainBeacon proof), and a
// from-scratch RebuildTrie reproduces the same root.
func TestBeaconUpdateDeterministicAndCommitted(t *testing.T) {
	prev := statetrie.ValueHash([]byte("parent")) // any 32-byte parent hash
	block := beaconBlock(1, prev, "salt-A-0123456789", "salt-B-abcdefghij")

	var seed1 []byte
	db := newTestDB(t)
	if err := db.Update(func(txn *bbolt.Tx) error {
		updateBeacon(txn, nil, block)

		seed1 = append([]byte{}, getBeacon(txn)...)
		if bytes.Equal(seed1, zeros32()) {
			t.Fatal("beacon did not advance off zero after a post-genesis block")
		}

		// committed & provable under StateRoot
		root, path, value, valueHash, proof, err := StateProof(txn, "beacon", ngtypes.BeaconStateKey)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(value, seed1) {
			t.Fatalf("proof value %x != beacon seed %x", value, seed1)
		}
		if !statetrie.Verify(root, path, valueHash, proof) {
			t.Fatal("DomainBeacon inclusion proof does not verify against StateRoot")
		}

		// the incremental root must equal a from-scratch rebuild
		incRoot := append([]byte{}, StateRoot(txn)...)
		if err := RebuildTrie(txn); err != nil {
			t.Fatal(err)
		}
		if rebuilt := StateRoot(txn); !bytes.Equal(incRoot, rebuilt) {
			t.Fatalf("rebuilt root %x != incremental root %x", rebuilt, incRoot)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// determinism: the identical block in a fresh db yields the identical seed
	db2 := newTestDB(t)
	if err := db2.Update(func(txn *bbolt.Tx) error {
		updateBeacon(txn, nil, beaconBlock(1, prev, "salt-A-0123456789", "salt-B-abcdefghij"))
		if seed2 := getBeacon(txn); !bytes.Equal(seed1, seed2) {
			t.Fatalf("beacon not deterministic: %x != %x", seed1, seed2)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestBeaconSaltsChangeSeed: different revealed salts must yield a different seed
// (the reveal entropy actually feeds the beacon), while a reveal-less block still
// advances it (via the parent hash / height terms).
func TestBeaconSaltsChangeSeed(t *testing.T) {
	prev := statetrie.ValueHash([]byte("parent"))

	seedOf := func(salts ...string) []byte {
		db := newTestDB(t)
		var out []byte
		if err := db.Update(func(txn *bbolt.Tx) error {
			updateBeacon(txn, nil, beaconBlock(1, prev, salts...))
			out = append([]byte{}, getBeacon(txn)...)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}

	a := seedOf("salt-one-0123456")
	b := seedOf("salt-two-0123456")
	none := seedOf()

	if bytes.Equal(a, b) {
		t.Fatal("distinct salts produced the same beacon seed")
	}
	if bytes.Equal(none, zeros32()) {
		t.Fatal("a reveal-less block left the beacon at zero (should still advance)")
	}
	if bytes.Equal(none, a) {
		t.Fatal("a reveal-less block matched a salted block's seed")
	}
}

// TestBeaconReorgUnwind: with a changeset recorder, reverting a height restores
// the exact prior seed and StateRoot — including unwinding the FIRST post-genesis
// beacon back to a beacon-free (absent) leaf.
func TestBeaconReorgUnwind(t *testing.T) {
	db := newTestDB(t)
	prev := statetrie.ValueHash([]byte("parent"))

	var rootBeforeH1, seedAtH1 []byte
	if err := db.Update(func(txn *bbolt.Tx) error {
		rootBeforeH1 = append([]byte{}, StateRoot(txn)...)

		// apply height 1 with a recorder (archive), capturing the absent pre-image
		cs1 := newChangeset(1)
		updateBeacon(txn, cs1, beaconBlock(1, prev, "h1-salt-01234567"))
		seedAtH1 = append([]byte{}, getBeacon(txn)...)
		rootAtH1 := append([]byte{}, StateRoot(txn)...)
		if bytes.Equal(rootAtH1, rootBeforeH1) {
			t.Fatal("StateRoot did not change when the first beacon was set")
		}

		// apply height 2 on top, then unwind it: back to seed@1 and root@1
		cs2 := newChangeset(2)
		updateBeacon(txn, cs2, beaconBlock(2, seedAtH1, "h2-salt-01234567"))
		if bytes.Equal(getBeacon(txn), seedAtH1) {
			t.Fatal("height 2 did not advance the beacon")
		}
		unwindHeightTxn(txn, 2)
		if got := getBeacon(txn); !bytes.Equal(got, seedAtH1) {
			t.Fatalf("after unwinding h2, beacon = %x, want seed@1 %x", got, seedAtH1)
		}
		if got := StateRoot(txn); !bytes.Equal(got, rootAtH1) {
			t.Fatalf("after unwinding h2, root = %x, want root@1 %x", got, rootAtH1)
		}

		// now unwind height 1: the beacon returns to absent and the root to genesis
		unwindHeightTxn(txn, 1)
		if got := getBeacon(txn); !bytes.Equal(got, zeros32()) {
			t.Fatalf("after unwinding h1, beacon = %x, want zeros (absent)", got)
		}
		if b := txn.Bucket(storage.BeaconBucketName).Get(ngtypes.BeaconStateKey); b != nil {
			t.Fatalf("after unwinding h1, beacon leaf still present: %x", b)
		}
		if got := StateRoot(txn); !bytes.Equal(got, rootBeforeH1) {
			t.Fatalf("after unwinding h1, root = %x, want pre-beacon root %x", got, rootBeforeH1)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
