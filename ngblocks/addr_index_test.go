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
