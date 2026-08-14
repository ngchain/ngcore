package ngstate

import (
	"encoding/binary"
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

		vm, err := NewVM(txn, acc, fakeTransactTx(nil, nil), 1)
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

		vm, err := NewVM(txn, acc, fakeTransactTx(nil, nil), 1)
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

		vm, err := NewVM(txn, contractAcc, fakeTransactTx(nil, nil), 1)
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

		vm, err := NewVM(txn, acc, fakeTransactTx(nil, nil), 1)
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
		if err := state.handleLock(txn, lockTx, 1); err != nil {
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
		if err := state.handleLock(txn, lockTx, 1); err == nil {
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
		if err := state.handleLock(txn, lockTx, 1); err != nil {
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
		if err := state.handleLock(txn, lockTx(600, privLev), 1); err == nil {
			t.Fatal("locking with an inactive dependency must fail")
		}

		// dex first, then leverage: the reference gets pinned
		if err := state.handleLock(txn, lockTx(500, privDex), 1); err != nil {
			t.Fatalf("lock dex: %v", err)
		}
		if err := state.handleLock(txn, lockTx(600, privLev), 1); err != nil {
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
		vm, err := NewVM(txn, levAcc, fakeTransactTx(nil, nil), 1)
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

// tokenWat is a shared-ledger service (erc20-style): balances live in
// the TOKEN's own kv, keyed by the 8-byte LE account num. transfer
// debits the CALLER (account.get_caller = msg.sender)
const tokenWat = `
(module
  (import "account" "get_caller" (func $caller (result i64)))
  (import "kv" "get" (func $get (param i32 i32 i32) (result i32)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  ;; layout: 0..8 from-key, 8..16 to-key, 16..24 from-bal, 24..32 to-bal
  (func (export "transfer") (param $to i64) (param $amount i64) (result i32)
    (i64.store (i32.const 0) (call $caller))
    (i64.store (i32.const 8) (local.get $to))
    (i64.store (i32.const 16) (i64.const 0))
    (i64.store (i32.const 24) (i64.const 0))
    (drop (call $get (i32.const 0) (i32.const 8) (i32.const 16)))
    (drop (call $get (i32.const 8) (i32.const 8) (i32.const 24)))
    (if (i64.lt_u (i64.load (i32.const 16)) (local.get $amount))
      (then (return (i32.const 0))))
    (i64.store (i32.const 16) (i64.sub (i64.load (i32.const 16)) (local.get $amount)))
    (i64.store (i32.const 24) (i64.add (i64.load (i32.const 24)) (local.get $amount)))
    (drop (call $set (i32.const 0) (i32.const 8) (i32.const 16) (i32.const 8)))
    (drop (call $set (i32.const 8) (i32.const 8) (i32.const 24) (i32.const 8)))
    (i32.const 1))
  (func (export "mint_to") (param $to i64) (param $amount i64)
    (i64.store (i32.const 8) (local.get $to))
    (i64.store (i32.const 24) (i64.const 0))
    (drop (call $get (i32.const 8) (i32.const 8) (i32.const 24)))
    (i64.store (i32.const 24) (i64.add (i64.load (i32.const 24)) (local.get $amount)))
    (drop (call $set (i32.const 8) (i32.const 8) (i32.const 24) (i32.const 8)))))
`

// tokenUserWat consumes the token service: mints itself 100 units and
// sends 30 to account 1 — all bookkeeping happens inside the token
const tokenUserWat = `
(module
  (import "service/700" "mint_to" (func $mint (param i64 i64)))
  (import "service/700" "transfer" (func $transfer (param i64 i64) (result i32)))
  (func (export "main")
    (call $mint (i64.const 600) (i64.const 100))
    (drop (call $transfer (i64.const 1) (i64.const 30)))))
`

// TestServiceToken covers own-state (service) semantics: the token's
// ledger lives in the token account's kv and is shared by all callers,
// with get_caller authorizing the debit
func TestServiceToken(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	privToken, _ := secp256k1.GeneratePrivateKey()
	privUser, _ := secp256k1.GeneratePrivateKey()

	err := db.Update(func(txn *bbolt.Tx) error {
		token := ngtypes.NewAccount(700, ngtypes.NewAddress(privToken), []byte(tokenWat), nil)
		putAccount(t, txn, token, 100)
		user := ngtypes.NewAccount(600, ngtypes.NewAddress(privUser), []byte(tokenUserWat), nil)
		putAccount(t, txn, user, 100)

		lock := func(convener uint64, priv *secp256k1.PrivateKey) error {
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.LockTx, 1, ngtypes.AccountNum(convener),
				nil, nil, big.NewInt(1), nil, nil)
			if err := tx.Signature(priv); err != nil {
				t.Fatal(err)
			}
			return state.handleLock(txn, tx, 1)
		}

		if err := lock(700, privToken); err != nil {
			t.Fatalf("lock token: %v", err)
		}
		if err := lock(600, privUser); err != nil {
			t.Fatalf("lock token user: %v", err)
		}

		// the service dependency pins the token like a library dep does
		tokenAcc, _ := getAccountByNum(txn, 700)
		if getRefCount(tokenAcc) != 1 {
			t.Fatalf("token refcount = %d, want 1", getRefCount(tokenAcc))
		}

		// run the consumer: the ledger updates happen in the TOKEN's kv
		userAcc, _ := getAccountByNum(txn, 600)
		vm, err := NewVM(txn, userAcc, fakeTransactTx(nil, nil), 1)
		if err != nil {
			t.Fatalf("NewVM with service dep: %v", err)
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("service run: %v", err)
		}

		tokenAcc, _ = getAccountByNum(txn, 700)
		key := func(num uint64) string {
			raw := make([]byte, 8)
			binary.LittleEndian.PutUint64(raw, num)
			return string(raw)
		}
		balOf := func(num uint64) uint64 {
			raw := tokenAcc.Context.Get(key(num))
			if len(raw) != 8 {
				t.Fatalf("token ledger entry for %d missing: %v", num, raw)
			}
			return binary.LittleEndian.Uint64(raw)
		}

		if got := balOf(600); got != 70 {
			t.Fatalf("token bal[600] = %d, want 70", got)
		}
		if got := balOf(1); got != 30 {
			t.Fatalf("token bal[1] = %d, want 30", got)
		}

		// the consumer's own kv stays empty (reserved keys aside): the
		// ledger lived in the callee
		userAcc, _ = getAccountByNum(txn, 600)
		for _, k := range userAcc.Context.Keys {
			if !strings.HasPrefix(k, "_") {
				t.Fatalf("user context leaked a ledger key: %q", k)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// u256Wat exercises the wide-integer extension: (2^128 - 1) + 1 with
// full carry propagation across the 64-bit limbs, the 32-byte result
// stored into kv
const u256Wat = `
(module
  (import "u256" "add" (func $add256 (param i32 i32 i32)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "sum")
  (data (i32.const 32) "\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff")
  (data (i32.const 64) "\01")
  (func (export "main")
    (call $add256 (i32.const 96) (i32.const 32) (i32.const 64))
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 96) (i32.const 32)))))
`

// TestVMU256: contracts can do 256-bit arithmetic through the wideint
// host modules — the basis for evm-scale token amounts
func TestVMU256(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewAccount(500, testAddr(0xaa), []byte(u256Wat), nil)
		acc.SetLock(true)
		putAccount(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(nil, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		reloaded, err := getAccountByNum(txn, 500)
		if err != nil {
			return err
		}

		// (2^128 - 1) + 1 = 2^128: LE byte 16 is 1, all others 0
		got := reloaded.Context.Get("sum")
		if len(got) != 32 {
			t.Fatalf("sum length = %d, want 32", len(got))
		}
		for i, b := range got {
			want := byte(0)
			if i == 16 {
				want = 1
			}
			if b != want {
				t.Fatalf("sum[%d] = %#x, want %#x (carry must cross the limbs)", i, b, want)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// txCtxWat snapshots the execution context into kv: the block
// timestamp, the amount this tx paid to the contract, and the
// remaining gas
const txCtxWat = `
(module
  (import "tx" "get_timestamp" (func $ts (result i64)))
  (import "tx" "get_paid_size" (func $paid_size (result i32)))
  (import "tx" "get_paid" (func $paid (param i32) (result i32)))
  (import "env" "get_gas" (func $gas (result i64)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "ts")
  (data (i32.const 2) "pd")
  (data (i32.const 4) "gs")
  (func (export "main")
    (i64.store (i32.const 16) (call $ts))
    (drop (call $set (i32.const 0) (i32.const 2) (i32.const 16) (i32.const 8)))
    (drop (call $set (i32.const 2) (i32.const 2) (i32.const 32) (call $paid (i32.const 32))))
    (i64.store (i32.const 16) (call $gas))
    (drop (call $set (i32.const 4) (i32.const 2) (i32.const 16) (i32.const 8)))))
`

// TestVMTxContext covers the timestamp / msg.value / gas introspection
func TestVMTxContext(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		owner := testAddr(0xaa)
		acc := ngtypes.NewAccount(500, owner, []byte(txCtxWat), nil)
		acc.SetLock(true)
		putAccount(t, txn, acc, 0)

		// the tx pays 77 to the contract's owner (msg.value) in two legs
		tx := fakeTransactTx(
			[]ngtypes.Address{owner, testAddr(0xbb), owner},
			[]*big.Int{big.NewInt(70), big.NewInt(5), big.NewInt(7)},
		)

		vm, err := NewVM(txn, acc, tx, 1755264000) // block timestamp
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		reloaded, err := getAccountByNum(txn, 500)
		if err != nil {
			return err
		}

		ts := reloaded.Context.Get("ts")
		if len(ts) != 8 || binary.LittleEndian.Uint64(ts) != 1755264000 {
			t.Fatalf("ts = %x, want LE(1755264000)", ts)
		}

		// 70 + 7 to the owner; the 5 to another address must not count
		paid := reloaded.Context.Get("pd")
		if len(paid) != 1 || paid[0] != 77 {
			t.Fatalf("paid = %x, want [77]", paid)
		}

		gas := reloaded.Context.Get("gs")
		if len(gas) != 8 {
			t.Fatalf("gas entry length = %d", len(gas))
		}
		remaining := binary.LittleEndian.Uint64(gas)
		if remaining == 0 || remaining >= vmMaxToll {
			t.Fatalf("gas remaining = %d, want within (0, %d)", remaining, int64(vmMaxToll))
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// kvScanWat writes entries under two prefixes, then sums the VALUES of
// every "b:"-prefixed entry by iterating keys — proving deterministic
// prefix enumeration
const kvScanWat = `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (import "kv" "get" (func $get (param i32 i32 i32) (result i32)))
  (import "kv" "count" (func $count (param i32 i32) (result i32)))
  (import "kv" "key_at" (func $key_at (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  ;; data: keys "a:x"=1 "b:p"=5 "b:q"=7 ; scratch: key buf 64, val buf 96, sum key "sum" at 16
  (data (i32.const 0) "a:xb:pb:qb")
  (data (i32.const 16) "sum")
  (func $put (param $kptr i32) (param $v i64)
    (i64.store (i32.const 96) (local.get $v))
    (drop (call $set (local.get $kptr) (i32.const 3) (i32.const 96) (i32.const 8))))
  (func (export "main")
    (local $i i32) (local $n i32) (local $sum i64)
    (call $put (i32.const 0) (i64.const 1))
    (call $put (i32.const 3) (i64.const 5))
    (call $put (i32.const 6) (i64.const 7))
    ;; iterate prefix "b:" (bytes at 9..10 would be "b" only; reuse 3..5 "b:")
    (local.set $n (call $count (i32.const 3) (i32.const 2)))
    (block $done
      (loop $next
        (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
        (drop (call $key_at (i32.const 3) (i32.const 2) (local.get $i) (i32.const 64)))
        (i64.store (i32.const 96) (i64.const 0))
        (drop (call $get (i32.const 64) (i32.const 3) (i32.const 96)))
        (local.set $sum (i64.add (local.get $sum) (i64.load (i32.const 96))))
        (local.set $i (i32.add (local.get $i) (i32.const 1)))
        (br $next)))
    (i64.store (i32.const 96) (local.get $sum))
    (drop (call $set (i32.const 16) (i32.const 3) (i32.const 96) (i32.const 8)))))
`

// TestVMKVScan: contracts can enumerate their kv by prefix
func TestVMKVScan(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewAccount(500, testAddr(0xaa), []byte(kvScanWat), nil)
		acc.SetLock(true)
		putAccount(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(nil, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		reloaded, err := getAccountByNum(txn, 500)
		if err != nil {
			return err
		}

		// sum of the "b:" entries = 5 + 7; the "a:" entry is excluded
		got := reloaded.Context.Get("sum")
		if len(got) != 8 || binary.LittleEndian.Uint64(got) != 12 {
			t.Fatalf("sum = %x, want LE(12)", got)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// emitWat emits two events and writes one kv entry
const emitWat = `
(module
  (import "log" "emit" (func $emit (param i32 i32 i32 i32) (result i32)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "transferdata1mint")
  (func (export "main")
    (drop (call $emit (i32.const 0) (i32.const 8) (i32.const 8) (i32.const 5)))
    (drop (call $emit (i32.const 13) (i32.const 4) (i32.const 0) (i32.const 0)))
    (drop (call $set (i32.const 0) (i32.const 8) (i32.const 8) (i32.const 5)))))
`

// bigVaultWat is a service accumulating a 256-bit total: the amount
// arrives through transfer slot 0 (32 bytes), the new total returns
// the same way — u256 crossing the service boundary
const bigVaultWat = `
(module
  (import "env" "buf_get" (func $bget (param i32 i32) (result i32)))
  (import "env" "buf_set" (func $bset (param i32 i32 i32) (result i32)))
  (import "kv" "get" (func $kvget (param i32 i32 i32) (result i32)))
  (import "kv" "set" (func $kvset (param i32 i32 i32 i32) (result i32)))
  (import "u256" "add" (func $add256 (param i32 i32 i32)))
  (memory 1)
  (data (i32.const 0) "tot")
  ;; 32: incoming amount, 64: stored total, 96: new total
  (func (export "deposit_big")
    (drop (call $bget (i32.const 0) (i32.const 32)))
    (drop (call $kvget (i32.const 0) (i32.const 3) (i32.const 64)))
    (call $add256 (i32.const 96) (i32.const 64) (i32.const 32))
    (drop (call $kvset (i32.const 0) (i32.const 3) (i32.const 96) (i32.const 32)))
    (drop (call $bset (i32.const 0) (i32.const 96) (i32.const 32)))))
`

// bigCallerWat deposits (2^128 - 1) twice: the second call must carry
// the full 256-bit total back across the boundary
func bigCallerWat() string {
	return `
(module
  (import "env" "buf_set" (func $bset (param i32 i32 i32) (result i32)))
  (import "env" "buf_get" (func $bget (param i32 i32) (result i32)))
  (import "kv" "set" (func $kvset (param i32 i32 i32 i32) (result i32)))
  (import "service/700" "deposit_big" (func $deposit))
  (memory 1)
  (data (i32.const 0) "got")
  (data (i32.const 32) "\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff\ff")
  (func (export "main")
    (drop (call $bset (i32.const 0) (i32.const 32) (i32.const 32)))
    (call $deposit)
    (drop (call $bset (i32.const 0) (i32.const 32) (i32.const 32)))
    (call $deposit)
    ;; read the final total from slot 0 into 64.. and store it
    (drop (call $bget (i32.const 0) (i32.const 64)))
    (drop (call $kvset (i32.const 0) (i32.const 3) (i32.const 64) (i32.const 32)))))
`
}

// TestServiceBigValues: 256-bit values cross the service boundary via
// the transfer slots, with carries intact
func TestServiceBigValues(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		vault := ngtypes.NewAccount(700, testAddr(0xaa), []byte(bigVaultWat), nil)
		vault.SetLock(true)
		putAccount(t, txn, vault, 0)

		caller := ngtypes.NewAccount(730, testAddr(0xdd), []byte(bigCallerWat()), nil)
		caller.SetLock(true)
		putAccount(t, txn, caller, 0)

		vm, err := NewVM(txn, caller, fakeTransactTx(nil, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		// 2 * (2^128 - 1) = 2^129 - 2: LE bytes fe ff*15 then byte16=01
		want := make([]byte, 32)
		want[0] = 0xfe
		for i := 1; i < 16; i++ {
			want[i] = 0xff
		}
		want[16] = 0x01

		callerAcc, _ := getAccountByNum(txn, 730)
		got := callerAcc.Context.Get("got")
		if len(got) != 32 {
			t.Fatalf("got length = %d, want 32", len(got))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got[%d] = %#x, want %#x (full total: %x)", i, got[i], want[i], got)
			}
		}

		// the vault's own kv holds the same total
		vaultAcc, _ := getAccountByNum(txn, 700)
		tot := vaultAcc.Context.Get("tot")
		for i := range want {
			if tot[i] != want[i] {
				t.Fatalf("vault tot mismatch at %d: %x", i, tot)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestReceiptsAndEvents: contract runs land in the local receipt with
// their events; failed runs record the failure without events
func TestReceiptsAndEvents(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewAccount(500, testAddr(0xaa), []byte(emitWat), nil)
		acc.SetLock(true)
		putAccount(t, txn, acc, 0)

		tx := fakeTransactTx(nil, nil)
		state.runContract(txn, 500, tx, VMEntryOnTx, 1)

		runs, err := GetTxRuns(txn, tx.GetHash())
		if err != nil {
			return err
		}
		if len(runs) != 1 {
			t.Fatalf("runs = %d, want 1", len(runs))
		}
		run := runs[0]
		if !run.Ok || run.Account != 500 || run.Entry != VMEntryOnTx {
			t.Fatalf("run = %+v", run)
		}
		if run.GasUsed == 0 {
			t.Fatal("run must report gas")
		}
		if len(run.Events) != 2 {
			t.Fatalf("events = %d, want 2", len(run.Events))
		}
		if run.Events[0].Topic != "transfer" || string(run.Events[0].Data) != "data1" ||
			run.Events[0].Contract != 500 {
			t.Fatalf("event[0] = %+v", run.Events[0])
		}
		if run.Events[1].Topic != "mint" || len(run.Events[1].Data) != 0 {
			t.Fatalf("event[1] = %+v", run.Events[1])
		}

		// a failing contract records the failure and drops its events
		bad := ngtypes.NewAccount(600, testAddr(0xbb), []byte(burnWat), nil)
		bad.SetLock(true)
		putAccount(t, txn, bad, 0)

		badTx := fakeTransactTx([]ngtypes.Address{testAddr(0xbb)}, []*big.Int{big.NewInt(0)})
		state.runContract(txn, 600, badTx, VMEntryOnTx, 1)

		badRuns, err := GetTxRuns(txn, badTx.GetHash())
		if err != nil {
			return err
		}
		if len(badRuns) != 1 || badRuns[0].Ok || badRuns[0].Error == "" {
			t.Fatalf("bad runs = %+v", badRuns)
		}
		if len(badRuns[0].Events) != 0 {
			t.Fatal("failed runs must not keep events")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestGasPricingTiers: storage writes must cost far more than pure
// computation, and the priced budget still aborts deterministically
func TestGasPricingTiers(t *testing.T) {
	db := newTestDB(t)

	run := func(wat string) uint64 {
		var gas uint64
		err := db.Update(func(txn *bbolt.Tx) error {
			acc := ngtypes.NewAccount(500, testAddr(0xaa), []byte(wat), nil)
			acc.SetLock(true)
			putAccount(t, txn, acc, 100)

			vm, err := NewVM(txn, acc, fakeTransactTx(nil, nil), 1)
			if err != nil {
				return err
			}
			gas, err = vm.DryRun(VMEntryOnTx)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return gas
	}

	// pure computation baseline
	pureWat := `
(module
  (func (export "main")
    (local $i i32)
    (local.set $i (i32.const 100))
    (block $out (loop $l
      (br_if $out (i32.eqz (local.get $i)))
      (local.set $i (i32.sub (local.get $i) (i32.const 1)))
      (br $l)))))
`
	pure := run(pureWat)
	write := run(kvWat) // one kv.set of a 3-byte key + 3-byte value

	// the tier must dominate: a single 6-byte write outweighs a
	// hundred-iteration compute loop
	wantMin := uint64(gasKVSetBase + gasKVSetPerByte*6)
	if write < wantMin {
		t.Fatalf("kv.set gas %d, want at least the tier %d", write, wantMin)
	}
	if write <= pure {
		t.Fatalf("one kv.set (%d) must cost more than the pure loop (%d)", write, pure)
	}
}

// TestVMDryRun: a dry run executes fully (gas burned, result visible)
// but never touches the chain state
func TestVMDryRun(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewAccount(500, testAddr(0xaa), []byte(kvWat), nil)
		acc.SetLock(true)
		putAccount(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(nil, nil), 1)
		if err != nil {
			return err
		}

		gasUsed, err := vm.DryRun(VMEntryOnTx)
		if err != nil {
			t.Fatalf("dry run failed: %v", err)
		}
		if gasUsed == 0 {
			t.Fatal("dry run must report burned gas")
		}

		// the kv write the contract performed must NOT be visible
		reloaded, err := getAccountByNum(txn, 500)
		if err != nil {
			return err
		}
		if got := reloaded.Context.Get("key"); len(got) != 0 {
			t.Fatalf("dry run leaked state: key=%q", got)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestNamedContractDeps: contracts are addressable as deployer.name —
// the name registers at lock time, imports resolve through the registry,
// conflicts are refused, and destroy releases the name
func TestNamedContractDeps(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	privToken, _ := secp256k1.GeneratePrivateKey()
	privUser, _ := secp256k1.GeneratePrivateKey()
	tokenDeployer := ngtypes.NewAddress(privToken)

	// the consumer imports the token via <deployerBS58>.token
	namedUserWat := `
(module
  (import "service/` + tokenDeployer.String() + `.token" "mint_to" (func $mint (param i64 i64)))
  (func (export "main")
    (call $mint (i64.const 600) (i64.const 5))))
`

	err := db.Update(func(txn *bbolt.Tx) error {
		token := ngtypes.NewAccount(700, tokenDeployer, []byte(tokenWat), nil)
		putAccount(t, txn, token, 100)
		user := ngtypes.NewAccount(600, ngtypes.NewAddress(privUser), []byte(namedUserWat), nil)
		putAccount(t, txn, user, 100)

		lock := func(convener uint64, priv *secp256k1.PrivateKey, name string) error {
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.LockTx, 1, ngtypes.AccountNum(convener),
				nil, nil, big.NewInt(1), []byte(name), nil)
			if err := tx.Signature(priv); err != nil {
				t.Fatal(err)
			}
			return state.handleLock(txn, tx, 1)
		}

		// the consumer cannot lock before the name exists
		if err := lock(600, privUser, ""); err == nil {
			t.Fatal("locking against an unregistered name must fail")
		}

		// lock the token under its name, then the consumer resolves it
		if err := lock(700, privToken, "token"); err != nil {
			t.Fatalf("lock named token: %v", err)
		}
		if err := lock(600, privUser, ""); err != nil {
			t.Fatalf("lock consumer: %v", err)
		}

		// the registry pinned the dependency by resolved num
		tokenAcc, _ := getAccountByNum(txn, 700)
		if getRefCount(tokenAcc) != 1 {
			t.Fatalf("token refcount = %d, want 1", getRefCount(tokenAcc))
		}

		// named execution works end to end
		userAcc, _ := getAccountByNum(txn, 600)
		vm, err := NewVM(txn, userAcc, fakeTransactTx(nil, nil), 1)
		if err != nil {
			t.Fatalf("NewVM with named dep: %v", err)
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("named service run: %v", err)
		}

		// another account of the SAME deployer cannot take the name
		other := ngtypes.NewAccount(701, tokenDeployer, []byte(dexWat), nil)
		putAccount(t, txn, other, 100)
		if err := lock(701, privToken, "token"); !errors.Is(err, ErrNameTaken) {
			t.Fatalf("name conflict: got %v, want ErrNameTaken", err)
		}

		// a DIFFERENT deployer may use the same name (separate namespace)
		otherDeployer, _ := secp256k1.GeneratePrivateKey()
		foreign := ngtypes.NewAccount(702, ngtypes.NewAddress(otherDeployer), []byte(dexWat), nil)
		putAccount(t, txn, foreign, 100)
		if err := lock(702, otherDeployer, "token"); err != nil {
			t.Fatalf("same name under another deployer must work: %v", err)
		}

		// invalid names are rejected
		bad := ngtypes.NewAccount(703, ngtypes.NewAddress(privUser), []byte(dexWat), nil)
		putAccount(t, txn, bad, 100)
		if err := lock(703, privUser, "Bad.Name!"); !errors.Is(err, ErrNameInvalid) {
			t.Fatalf("invalid name: got %v, want ErrNameInvalid", err)
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
		if err := state.handleLock(txn, lockTx, 1); err == nil {
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
