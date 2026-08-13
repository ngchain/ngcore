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

// --- tiny wasm assembler helpers, enough for the test contracts ---

func uleb(v uint32) []byte {
	var out []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if v == 0 {
			return out
		}
	}
}

func wasmVec(items ...[]byte) []byte {
	out := uleb(uint32(len(items)))
	for _, item := range items {
		out = append(out, item...)
	}
	return out
}

func wasmString(s string) []byte {
	return append(uleb(uint32(len(s))), s...)
}

func wasmSection(id byte, payload []byte) []byte {
	return append(append([]byte{id}, uleb(uint32(len(payload)))...), payload...)
}

func wasmModule(sections ...[]byte) []byte {
	out := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00} // magic + version
	for _, s := range sections {
		out = append(out, s...)
	}
	return out
}

func funcType(params, results []byte) []byte {
	out := []byte{0x60}
	out = append(out, uleb(uint32(len(params)))...)
	out = append(out, params...)
	out = append(out, uleb(uint32(len(results)))...)
	out = append(out, results...)
	return out
}

func funcImport(mod, name string, typeIdx byte) []byte {
	out := wasmString(mod)
	out = append(out, wasmString(name)...)
	out = append(out, 0x00, typeIdx)
	return out
}

func funcBody(code []byte) []byte {
	body := append([]byte{0x00}, code...) // no locals
	return append(uleb(uint32(len(body))), body...)
}

const (
	i32 = 0x7f
	i64 = 0x7e
)

// kvWasm is:
//
//	(module
//	  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
//	  (memory 1) (data (i32.const 0) "keyval")
//	  (func (export "main")
//	    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))))
func kvWasm() []byte {
	return wasmModule(
		wasmSection(1, wasmVec(
			funcType([]byte{i32, i32, i32, i32}, []byte{i32}),
			funcType(nil, nil),
		)),
		wasmSection(2, wasmVec(funcImport("kv", "set", 0))),
		wasmSection(3, wasmVec([]byte{1})),
		wasmSection(5, wasmVec([]byte{0x00, 0x01})), // memory min=1
		wasmSection(7, wasmVec(append(wasmString("main"), 0x00, 0x01))),
		wasmSection(10, wasmVec(funcBody([]byte{
			0x41, 0x00, // i32.const 0 (key ptr)
			0x41, 0x03, // i32.const 3 (key len)
			0x41, 0x03, // i32.const 3 (val ptr)
			0x41, 0x03, // i32.const 3 (val len)
			0x10, 0x00, // call $set
			0x1a, // drop
			0x0b, // end
		}))),
		wasmSection(11, wasmVec(append(
			[]byte{0x00, 0x41, 0x00, 0x0b}, // (data (i32.const 0)
			append(uleb(6), "keyval"...)...,
		))),
	)
}

// transferWasm calls coin.transfer(to=1, value=10) once
func transferWasm() []byte {
	return wasmModule(
		wasmSection(1, wasmVec(
			funcType([]byte{i64, i64}, []byte{i32}),
			funcType(nil, nil),
		)),
		wasmSection(2, wasmVec(funcImport("coin", "transfer", 0))),
		wasmSection(3, wasmVec([]byte{1})),
		wasmSection(7, wasmVec(append(wasmString("main"), 0x00, 0x01))),
		wasmSection(10, wasmVec(funcBody([]byte{
			0x42, 0x01, // i64.const 1 (to account num)
			0x42, 0x0a, // i64.const 10 (value)
			0x10, 0x00, // call $transfer
			0x1a, // drop
			0x0b, // end
		}))),
	)
}

// burnWasm writes a kv entry and then spins forever, so the toll station
// must abort it and the kv write must be rolled back
func burnWasm() []byte {
	return wasmModule(
		wasmSection(1, wasmVec(
			funcType([]byte{i32, i32, i32, i32}, []byte{i32}),
			funcType(nil, nil),
		)),
		wasmSection(2, wasmVec(funcImport("kv", "set", 0))),
		wasmSection(3, wasmVec([]byte{1})),
		wasmSection(5, wasmVec([]byte{0x00, 0x01})),
		wasmSection(7, wasmVec(append(wasmString("main"), 0x00, 0x01))),
		wasmSection(10, wasmVec(funcBody([]byte{
			0x41, 0x00, 0x41, 0x03, 0x41, 0x03, 0x41, 0x03,
			0x10, 0x00, 0x1a, // kv.set("key", "val") then drop
			0x03, 0x40, // loop
			0x0c, 0x00, // br 0
			0x0b, // end loop
			0x0b, // end
		}))),
		wasmSection(11, wasmVec(append(
			[]byte{0x00, 0x41, 0x00, 0x0b},
			append(uleb(6), "keyval"...)...,
		))),
	)
}

// logWasm calls log.debug with "hello" from its data segment
func logWasm() []byte {
	return wasmModule(
		wasmSection(1, wasmVec(
			funcType([]byte{i32, i32}, nil),
			funcType(nil, nil),
		)),
		wasmSection(2, wasmVec(funcImport("log", "debug", 0))),
		wasmSection(3, wasmVec([]byte{1})),
		wasmSection(5, wasmVec([]byte{0x00, 0x01})),
		wasmSection(7, wasmVec(append(wasmString("main"), 0x00, 0x01))),
		wasmSection(10, wasmVec(funcBody([]byte{
			0x41, 0x00, // i32.const 0
			0x41, 0x05, // i32.const 5
			0x10, 0x00, // call $debug
			0x0b, // end
		}))),
		wasmSection(11, wasmVec(append(
			[]byte{0x00, 0x41, 0x00, 0x0b},
			append(uleb(5), "hello"...)...,
		))),
	)
}

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
		acc := ngtypes.NewAccount(500, testAddr(0xaa), logWasm(), nil)
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
		acc := ngtypes.NewAccount(500, testAddr(0xaa), kvWasm(), nil)
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
		contractAcc := ngtypes.NewAccount(500, testAddr(0xaa), transferWasm(), nil)
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
		acc := ngtypes.NewAccount(500, testAddr(0xaa), burnWasm(), nil)
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
		acc := ngtypes.NewAccount(600, addr, logWasm(), nil)
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
