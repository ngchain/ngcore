package ngstate

import (
	"bytes"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// TestRemoteFallback covers the lazy-fork read-through hook: local misses
// resolve remotely, materialize inside writable txns only, and the nil
// default changes nothing. The hook is restored to nil afterwards so no
// other test sees it.
func TestRemoteFallback(t *testing.T) {
	db := newTestDB(t)

	contractAddr := testAddr(0xf1)
	balanceAddr := testAddr(0xf2)
	zeroBalAddr := testAddr(0xf3)
	viewAddr := testAddr(0xf4)

	code := mustWat(logWat)

	SetRemoteFallback(func(addr ngtypes.Address) (*ngtypes.Contract, *big.Int, bool) {
		switch addr {
		case contractAddr:
			return ngtypes.NewContract(contractAddr, code, nil), big.NewInt(77), true
		case balanceAddr, viewAddr:
			return nil, big.NewInt(42), true
		case zeroBalAddr:
			return nil, big.NewInt(0), true
		default:
			return nil, nil, false
		}
	})
	t.Cleanup(func() { SetRemoteFallback(nil) })

	// read-only txn: the fetched value is served but NOT materialized
	err := db.View(func(txn *bbolt.Tx) error {
		if got := getBalance(txn, viewAddr); got.Int64() != 42 {
			t.Fatalf("view balance = %s, want 42", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(txn *bbolt.Tx) error {
		// the view read above must not have written anything
		if raw := txn.Bucket(storage.Addr2BalBucketName).Get(viewAddr[:]); raw != nil {
			t.Fatal("read-only fetch got materialized")
		}

		// a contract miss resolves remotely and materializes
		acc, err := getContract(txn, contractAddr)
		if err != nil {
			t.Fatalf("remote contract fetch: %v", err)
		}
		if !bytes.Equal(acc.Source, code) {
			t.Fatal("remote contract source lost")
		}
		if raw := txn.Bucket(storage.ContractBucketName).Get(contractAddr[:]); raw == nil {
			t.Fatal("remote contract not materialized in a writable txn")
		}
		if !contractExists(txn, contractAddr) {
			t.Fatal("contractExists must see the materialized contract")
		}

		// a balance-only miss materializes the balance
		if got := getBalance(txn, balanceAddr); got.Int64() != 42 {
			t.Fatalf("remote balance = %s, want 42", got)
		}
		if raw := txn.Bucket(storage.Addr2BalBucketName).Get(balanceAddr[:]); raw == nil {
			t.Fatal("remote balance not materialized")
		}

		// a zero balance is served but never written
		if got := getBalance(txn, zeroBalAddr); got.Sign() != 0 {
			t.Fatalf("zero remote balance = %s", got)
		}
		if raw := txn.Bucket(storage.Addr2BalBucketName).Get(zeroBalAddr[:]); raw != nil {
			t.Fatal("zero balance must not be materialized")
		}

		// negative results stay local misses
		if _, err := getContract(txn, testAddr(0xf9)); err == nil {
			t.Fatal("unknown address must still miss")
		}
		if contractExists(txn, testAddr(0xf9)) {
			t.Fatal("contractExists must miss on a negative result")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// back to the nil default: everything misses locally again
	SetRemoteFallback(nil)
	err = db.Update(func(txn *bbolt.Tx) error {
		if _, _, ok := fetchRemoteState(txn, testAddr(0xf8)); ok {
			t.Fatal("nil hook must never resolve")
		}
		if got := getBalance(txn, testAddr(0xf8)); got.Sign() != 0 {
			t.Fatalf("balance without hook = %s", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
