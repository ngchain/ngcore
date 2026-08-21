package ngblocks

import (
	"math/big"
	"testing"

	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// TestAddrTxIndex covers the account-history index: a tx is retrievable
// under both its sender and recipient, respects the height range and
// limit, and is removed on unindex
func TestAddrTxIndex(t *testing.T) {
	db := newDB(t)
	keyA := newKey(t)
	addrA := ngtypes.NewAddress(keyA)
	dest := ngtypes.NewAddress(newKey(t))

	xfer := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 5, dest, big.NewInt(1), big.NewInt(1), nil, nil)
	if err := xfer.Signature(keyA); err != nil {
		t.Fatal(err)
	}

	update(t, db, func(_, txBucket *bbolt.Bucket) error {
		raw, err := rlp.EncodeToBytes(xfer)
		if err != nil {
			return err
		}
		if err := txBucket.Put(xfer.GetHash(), raw); err != nil {
			return err
		}
		return putTxAddrIndex(txBucket, xfer)
	})

	get := func(addr ngtypes.Address, from, to uint64, limit int) []*ngtypes.FullTx {
		var out []*ngtypes.FullTx
		if err := db.View(func(txn *bbolt.Tx) error {
			var err error
			out, err = GetTxsByAddress(txn.Bucket(storage.TxBucketName), addr, from, to, limit)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}

	if got := get(addrA, 0, 0, 0); len(got) != 1 {
		t.Fatalf("sender history = %d, want 1", len(got))
	}
	if got := get(dest, 0, 0, 0); len(got) != 1 {
		t.Fatalf("recipient history = %d, want 1", len(got))
	}
	// height 5 falls outside [6, ∞) and [0, 4]
	if got := get(addrA, 6, 0, 0); len(got) != 0 {
		t.Fatalf("fromHeight 6 returned %d, want 0", len(got))
	}
	if got := get(addrA, 0, 4, 0); len(got) != 0 {
		t.Fatalf("toHeight 4 returned %d, want 0", len(got))
	}
	// a limit of exactly the count still returns it
	if got := get(dest, 0, 0, 1); len(got) != 1 {
		t.Fatalf("limit 1 returned %d, want 1", len(got))
	}

	// unindex clears both sides
	update(t, db, func(_, txBucket *bbolt.Bucket) error { return delTxAddrIndex(txBucket, xfer) })
	if got := get(addrA, 0, 0, 0); len(got) != 0 {
		t.Fatalf("after unindex sender history = %d, want 0", len(got))
	}
	if got := get(dest, 0, 0, 0); len(got) != 0 {
		t.Fatalf("after unindex recipient history = %d, want 0", len(got))
	}
}

// TestAddrTxIndexHeightOrder crosses the 256 boundary, where a little-endian
// height would interleave keys and break the range seek / height ordering:
// with big-endian heights the index stays numerically ordered
func TestAddrTxIndexHeightOrder(t *testing.T) {
	db := newDB(t)
	keyA := newKey(t)
	addrA := ngtypes.NewAddress(keyA)
	dest := ngtypes.NewAddress(newKey(t))

	heights := []uint64{1, 100, 256, 300}
	update(t, db, func(_, txBucket *bbolt.Bucket) error {
		for _, h := range heights {
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, h, dest, big.NewInt(1), big.NewInt(1), nil, nil)
			if err := tx.Signature(keyA); err != nil {
				return err
			}
			raw, err := rlp.EncodeToBytes(tx)
			if err != nil {
				return err
			}
			if err := txBucket.Put(tx.GetHash(), raw); err != nil {
				return err
			}
			if err := putTxAddrIndex(txBucket, tx); err != nil {
				return err
			}
		}
		return nil
	})

	got := func(from, to uint64) []uint64 {
		var hs []uint64
		if err := db.View(func(txn *bbolt.Tx) error {
			out, err := GetTxsByAddress(txn.Bucket(storage.TxBucketName), addrA, from, to, 0)
			for _, tx := range out {
				hs = append(hs, tx.Height)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return hs
	}

	eq := func(name string, want []uint64, from, to uint64) {
		g := got(from, to)
		if len(g) != len(want) {
			t.Fatalf("%s = %v, want %v", name, g, want)
		}
		for i := range want {
			if g[i] != want[i] {
				t.Fatalf("%s = %v, want %v (order/height mismatch)", name, g, want)
			}
		}
	}

	eq("all", []uint64{1, 100, 256, 300}, 0, 0)
	eq("fromHeight 2", []uint64{100, 256, 300}, 2, 0)
	eq("toHeight 100", []uint64{1, 100}, 0, 100)
	eq("range [100,256]", []uint64{100, 256}, 100, 256)
}
