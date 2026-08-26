package statetrie_test

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/ngchain/ngcore/statetrie"
	"github.com/ngchain/ngcore/utils"
)

func TestEmptyRootDeterministic(t *testing.T) {
	r1 := statetrie.EmptyRoot()
	r2 := statetrie.Root(statetrie.NewMemStore())
	if !bytes.Equal(r1, r2) {
		t.Fatalf("empty root mismatch: %x vs %x", r1, r2)
	}
	if bytes.Equal(r1, make([]byte, 32)) {
		t.Fatal("empty root must not be zeros (it is the folded default chain)")
	}
	// pin the empty root: any change here is a consensus break
	t.Logf("empty root: %s", hex.EncodeToString(r1))
}

func TestInsertUpdateDeleteRoundTrip(t *testing.T) {
	store := statetrie.NewMemStore()
	empty := statetrie.Root(store)

	path := statetrie.LeafPath(statetrie.DomainBalance, bytes.Repeat([]byte{0xaa}, 32))
	v1 := statetrie.ValueHash([]byte{1, 2, 3})
	v2 := statetrie.ValueHash([]byte{4, 5, 6})

	if err := statetrie.Update(store, path, v1); err != nil {
		t.Fatal(err)
	}
	r1 := statetrie.Root(store)
	if bytes.Equal(r1, empty) {
		t.Fatal("insert must change the root")
	}

	if err := statetrie.Update(store, path, v2); err != nil {
		t.Fatal(err)
	}
	r2 := statetrie.Root(store)
	if bytes.Equal(r2, r1) {
		t.Fatal("update must change the root")
	}

	// back to v1 restores r1
	if err := statetrie.Update(store, path, v1); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(statetrie.Root(store), r1) {
		t.Fatal("re-setting the old value must restore the old root")
	}

	// delete restores the empty root AND the empty store
	if err := statetrie.Update(store, path, statetrie.ZeroHash()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(statetrie.Root(store), empty) {
		t.Fatal("delete must restore the empty root")
	}
	if store.Len() != 0 {
		t.Fatalf("delete must remove every stored node, %d left", store.Len())
	}
}

func TestOrderIndependence(t *testing.T) {
	type kv struct {
		path []byte
		val  []byte
	}
	rnd := rand.New(rand.NewSource(42))
	entries := make([]kv, 64)
	for i := range entries {
		raw := make([]byte, 32)
		rnd.Read(raw)
		entries[i] = kv{
			path: statetrie.LeafPath(statetrie.DomainContract, raw),
			val:  statetrie.ValueHash(raw),
		}
	}

	apply := func(order []int, withNoise bool) []byte {
		store := statetrie.NewMemStore()
		if withNoise {
			// interleave inserts that get deleted again: the FINAL SET alone
			// must decide the root
			for _, e := range entries[:8] {
				noise := statetrie.LeafPath(statetrie.DomainKey, e.val)
				if err := statetrie.Update(store, noise, e.val); err != nil {
					t.Fatal(err)
				}
				if err := statetrie.Update(store, noise, statetrie.ZeroHash()); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, i := range order {
			if err := statetrie.Update(store, entries[i].path, entries[i].val); err != nil {
				t.Fatal(err)
			}
		}
		return statetrie.Root(store)
	}

	order := make([]int, len(entries))
	for i := range order {
		order[i] = i
	}
	want := apply(order, false)

	for trial := 0; trial < 3; trial++ {
		rnd.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		if got := apply(order, trial == 2); !bytes.Equal(got, want) {
			t.Fatalf("root depends on op order: %x vs %x", got, want)
		}
	}
}

func TestProveVerify(t *testing.T) {
	store := statetrie.NewMemStore()
	rnd := rand.New(rand.NewSource(7))

	paths := make([][]byte, 16)
	vals := make([][]byte, 16)
	for i := range paths {
		raw := make([]byte, 32)
		rnd.Read(raw)
		paths[i] = statetrie.LeafPath(statetrie.DomainBalance, raw)
		vals[i] = statetrie.ValueHash(raw)
		if err := statetrie.Update(store, paths[i], vals[i]); err != nil {
			t.Fatal(err)
		}
	}
	root := statetrie.Root(store)

	for i := range paths {
		proof := statetrie.Prove(store, paths[i])
		if len(proof) != statetrie.Depth {
			t.Fatalf("proof must carry %d siblings, got %d", statetrie.Depth, len(proof))
		}
		if !statetrie.Verify(root, paths[i], vals[i], proof) {
			t.Fatalf("valid proof %d rejected", i)
		}

		// tampered value hash
		if statetrie.Verify(root, paths[i], statetrie.ValueHash([]byte("evil")), proof) {
			t.Fatal("tampered valueHash accepted")
		}
		// tampered sibling
		bad := make([][]byte, len(proof))
		copy(bad, proof)
		flipped := append([]byte{}, proof[13]...)
		flipped[0] ^= 1
		bad[13] = flipped
		if statetrie.Verify(root, paths[i], vals[i], bad) {
			t.Fatal("tampered proof accepted")
		}
		// wrong root
		if statetrie.Verify(utils.Hash256([]byte("no")), paths[i], vals[i], proof) {
			t.Fatal("wrong root accepted")
		}
		// truncated proof
		if statetrie.Verify(root, paths[i], vals[i], proof[:255]) {
			t.Fatal("truncated proof accepted")
		}
	}

	// proof of absence: an untouched path verifies against the zero hash
	absent := statetrie.LeafPath(statetrie.DomainBalance, []byte("nobody"))
	proof := statetrie.Prove(store, absent)
	if !statetrie.Verify(root, absent, statetrie.ZeroHash(), proof) {
		t.Fatal("absence proof rejected")
	}
	if statetrie.Verify(root, absent, statetrie.ValueHash([]byte("x")), proof) {
		t.Fatal("absence path verified a present value")
	}
}

func TestDeleteRestoresPriorRoot(t *testing.T) {
	store := statetrie.NewMemStore()
	rnd := rand.New(rand.NewSource(99))

	var roots [][]byte
	var paths [][]byte
	roots = append(roots, statetrie.Root(store))
	for i := 0; i < 20; i++ {
		raw := make([]byte, 32)
		rnd.Read(raw)
		p := statetrie.LeafPath(statetrie.DomainKey, raw)
		paths = append(paths, p)
		if err := statetrie.Update(store, p, statetrie.ValueHash(raw)); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, statetrie.Root(store))
	}

	// deleting in reverse walks the roots back exactly
	for i := len(paths) - 1; i >= 0; i-- {
		if err := statetrie.Update(store, paths[i], statetrie.ZeroHash()); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(statetrie.Root(store), roots[i]) {
			t.Fatalf("delete %d did not restore the prior root", i)
		}
	}
	if store.Len() != 0 {
		t.Fatalf("store must be empty after deleting everything, %d nodes left", store.Len())
	}
}

func TestDomainSeparation(t *testing.T) {
	raw := bytes.Repeat([]byte{0x11}, 32)
	pBal := statetrie.LeafPath(statetrie.DomainBalance, raw)
	pKey := statetrie.LeafPath(statetrie.DomainKey, raw)
	pCon := statetrie.LeafPath(statetrie.DomainContract, raw)
	if bytes.Equal(pBal, pKey) || bytes.Equal(pBal, pCon) || bytes.Equal(pKey, pCon) {
		t.Fatal("domains must map the same raw key to different leaf paths")
	}
}
