package ngstate

import (
	"math/big"
	"path/filepath"
	"testing"

	"github.com/ngchain/secp256k1"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// The on-chain contracts are plain wat text — human-readable and
// editable through append/delete txs

const kvWat = `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))))
`

const transferWat = `
(module
  (import "coin" "transfer" (func $transfer (param i64 i64) (result i32)))
  (func (export "main")
    (drop (call $transfer (i64.const 1) (i64.const 10)))))
`

// burnWat writes a kv entry and then spins forever, so the toll station
// must abort it and the kv write must be rolled back
const burnWat = `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))
    (loop $forever (br $forever))))
`

const logWat = `
(module
  (import "log" "debug" (func $debug (param i32 i32)))
  (memory 1)
  (data (i32.const 0) "hello")
  (func (export "main")
    (call $debug (i32.const 0) (i32.const 5))))
`

// --- test env helpers ---

func newTestDB(t *testing.T) *bbolt.DB {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "test.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storage.InitDB(db)

	return db
}

func testAddr(b byte) ngtypes.Address {
	var addr ngtypes.Address
	for i := range addr {
		addr[i] = b
	}
	return addr
}

// putAccount registers the account with its ownership and balance
func putAccount(t *testing.T, txn *bbolt.Tx, acc *ngtypes.Account, balance int64) {
	t.Helper()

	if err := setAccount(txn, ngtypes.AccountNum(acc.Num), acc); err != nil {
		t.Fatal(err)
	}
	if err := setOwnership(txn, acc.Owner, ngtypes.AccountNum(acc.Num)); err != nil {
		t.Fatal(err)
	}
	if err := setBalance(txn, acc.Owner, big.NewInt(balance)); err != nil {
		t.Fatal(err)
	}
}

func fakeTransactTx(participants []ngtypes.Address, values []*big.Int) *ngtypes.FullTx {
	return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1, 2,
		participants, values, big.NewInt(0), nil, nil)
}

// --- the tests ---

func TestVMLog(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewAccount(500, testAddr(0xaa), []byte(logWat), nil)
		acc.SetLock(true)
		putAccount(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(nil, nil))
		if err != nil {
			return err
		}

		return vm.Run(VMEntryOnTx)
	})
	if err != nil {
		t.Fatalf("log contract failed: %v", err)
	}
}

func TestVMKVSet(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewAccount(500, testAddr(0xaa), []byte(kvWat), nil)
		acc.SetLock(true)
		putAccount(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(nil, nil))
		if err != nil {
			return err
		}

		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		// reload from the db to prove the journal got flushed
		reloaded, err := getAccountByNum(txn, 500)
		if err != nil {
			return err
		}

		if got := string(reloaded.Context.Get("key")); got != "val" {
			t.Fatalf("kv.set not applied, got %q", got)
		}
		if !reloaded.IsLocked() {
			t.Fatal("the lock flag got lost by the contract flush")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVMTransfer(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		contractAcc := ngtypes.NewAccount(500, testAddr(0xaa), []byte(transferWat), nil)
		contractAcc.SetLock(true)
		putAccount(t, txn, contractAcc, 100)

		receiver := ngtypes.NewAccount(1, testAddr(0xbb), nil, nil)
		putAccount(t, txn, receiver, 0)

		vm, err := NewVM(txn, contractAcc, fakeTransactTx(nil, nil))
		if err != nil {
			return err
		}

		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		if got := getBalance(txn, testAddr(0xaa)); got.Int64() != 90 {
			t.Fatalf("sender balance = %d, want 90", got.Int64())
		}
		if got := getBalance(txn, testAddr(0xbb)); got.Int64() != 10 {
			t.Fatalf("receiver balance = %d, want 10", got.Int64())
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVMTollOverflowRollsBack(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewAccount(500, testAddr(0xaa), []byte(burnWat), nil)
		acc.SetLock(true)
		putAccount(t, txn, acc, 100)

		vm, err := NewVM(txn, acc, fakeTransactTx(nil, nil))
		if err != nil {
			return err
		}

		if err := vm.Run(VMEntryOnTx); err == nil {
			t.Fatal("infinite loop contract should be aborted by the toll station")
		}

		// the kv write before the loop must be gone
		reloaded, err := getAccountByNum(txn, 500)
		if err != nil {
			return err
		}
		if got := reloaded.Context.Get("key"); len(got) != 0 {
			t.Fatalf("journal leaked on a failed call: key=%q", got)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLockUnlockFlow(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := ngtypes.NewAddress(priv)

	err = db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewAccount(600, addr, []byte(logWat), nil)
		putAccount(t, txn, acc, 100)

		// lock the account: the vm becomes active, editing gets frozen
		lockTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.LockTx, 1, 600, nil, nil, big.NewInt(1), nil, nil)
		if err := lockTx.Signature(priv); err != nil {
			return err
		}
		if err := state.handleLock(txn, lockTx); err != nil {
			t.Fatalf("handleLock: %v", err)
		}

		locked, err := getAccountByNum(txn, 600)
		if err != nil {
			return err
		}
		if !locked.IsLocked() {
			t.Fatal("account should be locked")
		}
		if got := getBalance(txn, addr); got.Int64() != 99 {
			t.Fatalf("lock fee not charged, balance = %d", got.Int64())
		}

		// double lock must fail
		if err := state.handleLock(txn, lockTx); err == nil {
			t.Fatal("locking a locked account should fail")
		}

		// append on a locked account must fail
		appendTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.AppendTx, 1, 600, nil, nil, big.NewInt(1), nil, nil)
		if err := appendTx.Signature(priv); err != nil {
			return err
		}
		if err := state.handleAppend(txn, appendTx); err != ErrAccountLocked {
			t.Fatalf("append on locked account: got %v, want ErrAccountLocked", err)
		}

		// unlock reverts everything
		unlockTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.UnlockTx, 1, 600, nil, nil, big.NewInt(1), nil, nil)
		if err := unlockTx.Signature(priv); err != nil {
			return err
		}
		if err := state.handleUnlock(txn, unlockTx); err != nil {
			t.Fatalf("handleUnlock: %v", err)
		}

		unlocked, err := getAccountByNum(txn, 600)
		if err != nil {
			return err
		}
		if unlocked.IsLocked() {
			t.Fatal("account should be unlocked")
		}

		// double unlock must fail
		if err := state.handleUnlock(txn, unlockTx); err == nil {
			t.Fatal("unlocking an unlocked account should fail")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLockRejectsBrokenContract(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := ngtypes.NewAddress(priv)

	err = db.Update(func(txn *bbolt.Tx) error {
		// a half-edited contract text must not be lockable
		acc := ngtypes.NewAccount(700, addr, []byte(`(module (func (export "main")`), nil)
		putAccount(t, txn, acc, 100)

		lockTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.LockTx, 1, 700, nil, nil, big.NewInt(1), nil, nil)
		if err := lockTx.Signature(priv); err != nil {
			return err
		}

		if err := checkLock(txn, lockTx); err == nil {
			t.Fatal("checkLock should reject a non-compiling contract")
		}
		if err := state.handleLock(txn, lockTx); err == nil {
			t.Fatal("handleLock should reject a non-compiling contract")
		}

		reloaded, err := getAccountByNum(txn, 700)
		if err != nil {
			return err
		}
		if reloaded.IsLocked() {
			t.Fatal("account must stay unlocked after a failed lock")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
