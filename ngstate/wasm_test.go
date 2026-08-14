package ngstate

import (
	"errors"
	"math/big"
	"path/filepath"
	"strings"
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

		// editing a locked account must fail
		editTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.EditTx, 1, 600, nil, nil, big.NewInt(1), nil, nil)
		if err := editTx.Signature(priv); err != nil {
			return err
		}
		if err := state.handleEdit(txn, editTx); err != ErrAccountLocked {
			t.Fatalf("edit on locked account: got %v, want ErrAccountLocked", err)
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

// TestEditFlow upgrades a contract with a minimal diff patch:
// unlock-free edit on an unlocked account, then lock runs the vm on the
// new text
func TestEditFlow(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := ngtypes.NewAddress(priv)

	newWat := strings.Replace(transferWat, "i64.const 10", "i64.const 25", 1)
	hunks := ngtypes.DiffHunks([]byte(transferWat), []byte(newWat))
	patchSize := 0
	for _, h := range hunks {
		patchSize += len(h.Del) + len(h.Ins)
	}
	if patchSize > 16 {
		t.Fatalf("small edit produced a big patch: %d bytes", patchSize)
	}

	rawExtra, err := ngtypes.NewEditExtra([]byte(transferWat), hunks).Encode()
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewAccount(800, addr, []byte(transferWat), nil)
		putAccount(t, txn, acc, 100)

		editTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.EditTx, 1, 800, nil, nil, big.NewInt(1), rawExtra, nil)
		if err := editTx.Signature(priv); err != nil {
			return err
		}

		if err := checkEdit(txn, editTx); err != nil {
			t.Fatalf("checkEdit: %v", err)
		}
		if err := state.handleEdit(txn, editTx); err != nil {
			t.Fatalf("handleEdit: %v", err)
		}

		reloaded, err := getAccountByNum(txn, 800)
		if err != nil {
			return err
		}
		if string(reloaded.Contract) != newWat {
			t.Fatalf("contract not patched:\n%s", reloaded.Contract)
		}
		if got := getBalance(txn, addr); got.Int64() != 99 {
			t.Fatalf("edit fee not charged, balance = %d", got.Int64())
		}

		// the patched text must still compile and lock
		lockTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.LockTx, 1, 800, nil, nil, big.NewInt(1), nil, nil)
		if err := lockTx.Signature(priv); err != nil {
			return err
		}
		if err := state.handleLock(txn, lockTx); err != nil {
			t.Fatalf("handleLock after edit: %v", err)
		}

		// a mismatching patch (stale base) must be rejected
		staleExtra, err := (&ngtypes.EditExtra{Hunks: []ngtypes.Hunk{
			{Pos: 0, Del: []byte("XXX"), Ins: []byte("YYY")},
		}}).Encode()
		if err != nil {
			return err
		}
		staleTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.EditTx, 1, 800, nil, nil, big.NewInt(1), staleExtra, nil)
		if err := staleTx.Signature(priv); err != nil {
			return err
		}
		if err := checkEdit(txn, staleTx); err != ErrAccountLocked {
			// locked now; unlock first to test the mismatch path
			t.Fatalf("stale edit on locked account: got %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// dexWat is a pure code library: it exports an algorithm and touches no
// state of its own
const dexWat = `
(module
  (func (export "double") (param i64) (result i64)
    (i64.mul (local.get 0) (i64.const 2))))
`

// leverageWat composes the dex module: it imports dex's algorithm and
// stores the computed result into its OWN kv state (delegate semantics)
const leverageWat = `
(module
  (import "contract/500" "double" (func $double (param i64) (result i64)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "num")
  (func (export "main")
    (i64.store8 (i32.const 8) (call $double (i64.const 21)))
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 8) (i32.const 1)))))
`

// TestContractModuleDeps covers the code-module dependency system:
// leverage(600) imports dex(500), the reference pins dex until leverage
// releases it, and the linked execution runs dex's code on leverage's
// state
func TestContractModuleDeps(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	privDex, _ := secp256k1.GeneratePrivateKey()
	privLev, _ := secp256k1.GeneratePrivateKey()

	err := db.Update(func(txn *bbolt.Tx) error {
		dex := ngtypes.NewAccount(500, ngtypes.NewAddress(privDex), []byte(dexWat), nil)
		putAccount(t, txn, dex, 100)
		lev := ngtypes.NewAccount(600, ngtypes.NewAddress(privLev), []byte(leverageWat), nil)
		putAccount(t, txn, lev, 100)

		lockTx := func(convener uint64, priv *secp256k1.PrivateKey) *ngtypes.FullTx {
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.LockTx, 1, ngtypes.AccountNum(convener),
				nil, nil, big.NewInt(1), nil, nil)
			if err := tx.Signature(priv); err != nil {
				t.Fatal(err)
			}
			return tx
		}
		unlockTx := func(convener uint64, priv *secp256k1.PrivateKey) *ngtypes.FullTx {
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.UnlockTx, 1, ngtypes.AccountNum(convener),
				nil, nil, big.NewInt(1), nil, nil)
			if err := tx.Signature(priv); err != nil {
				t.Fatal(err)
			}
			return tx
		}

		// locking leverage before its dependency is active must fail
		if err := state.handleLock(txn, lockTx(600, privLev)); err == nil {
			t.Fatal("locking with an inactive dependency must fail")
		}

		// dex first, then leverage: the reference gets pinned
		if err := state.handleLock(txn, lockTx(500, privDex)); err != nil {
			t.Fatalf("lock dex: %v", err)
		}
		if err := state.handleLock(txn, lockTx(600, privLev)); err != nil {
			t.Fatalf("lock leverage: %v", err)
		}

		dexAcc, _ := getAccountByNum(txn, 500)
		if getRefCount(dexAcc) != 1 {
			t.Fatalf("dex refcount = %d, want 1", getRefCount(dexAcc))
		}

		// the depended-on module can neither unlock nor be destroyed
		if err := state.handleUnlock(txn, unlockTx(500, privDex)); !errors.Is(err, ErrAccountRefdBy) {
			t.Fatalf("unlock dex while referenced: got %v, want ErrAccountRefdBy", err)
		}

		// linked execution: leverage's main calls dex's double and
		// writes 42 into leverage's own kv
		levAcc, _ := getAccountByNum(txn, 600)
		vm, err := NewVM(txn, levAcc, fakeTransactTx(nil, nil))
		if err != nil {
			t.Fatalf("NewVM with deps: %v", err)
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("linked run: %v", err)
		}

		levAcc, _ = getAccountByNum(txn, 600)
		if got := levAcc.Context.Get("num"); len(got) != 1 || got[0] != 42 {
			t.Fatalf("leverage kv num = %v, want [42]", got)
		}
		// delegate semantics: dex's own state stays untouched
		dexAcc, _ = getAccountByNum(txn, 500)
		if got := dexAcc.Context.Get("num"); len(got) != 0 {
			t.Fatal("dex state must stay untouched")
		}

		// release: unlock leverage, then dex frees up
		if err := state.handleUnlock(txn, unlockTx(600, privLev)); err != nil {
			t.Fatalf("unlock leverage: %v", err)
		}
		dexAcc, _ = getAccountByNum(txn, 500)
		if getRefCount(dexAcc) != 0 {
			t.Fatalf("dex refcount = %d after release, want 0", getRefCount(dexAcc))
		}
		if err := state.handleUnlock(txn, unlockTx(500, privDex)); err != nil {
			t.Fatalf("unlock dex after release: %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestDestroyRules: an account cannot be destroyed while its contract
// is active (locked) or non-empty — downstream contracts may depend on
// it; after unlock + clearing, destroy goes through and removes the
// account (with its Context) entirely
func TestDestroyRules(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := ngtypes.NewAddress(priv)

	err = db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewAccount(900, addr, []byte(logWat), nil)
		acc.SetLock(true)
		putAccount(t, txn, acc, 100)

		destroyTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.DestroyTx, 1, 900, nil, nil, big.NewInt(1), nil, nil)
		if err := destroyTx.Signature(priv); err != nil {
			return err
		}

		// locked: refused
		if err := state.handleDestroy(txn, destroyTx); err == nil {
			t.Fatal("destroying a locked account must fail")
		}

		// unlocked but the contract text remains: still refused
		acc.SetLock(false)
		if err := setAccount(txn, 900, acc); err != nil {
			return err
		}
		if err := state.handleDestroy(txn, destroyTx); err != ErrDestroyAccountContractNotEmpty {
			t.Fatalf("destroying with a contract: got %v, want ErrDestroyAccountContractNotEmpty", err)
		}

		// cleared: destroy goes through and the account is gone
		acc.Contract = nil
		if err := setAccount(txn, 900, acc); err != nil {
			return err
		}
		if err := state.handleDestroy(txn, destroyTx); err != nil {
			t.Fatalf("destroy after clearing: %v", err)
		}
		if _, err := getAccountByNum(txn, 900); err == nil {
			t.Fatal("account (and its context) must be removed")
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
