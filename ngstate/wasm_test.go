package ngstate

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// The on-chain contracts are plain wat text — human-readable and
// editable through append/delete txs. Identities are 32-byte addresses
// passed through linear memory (and buf slots across service frames)

const kvWat = `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "keyval")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 3)))))
`

// watBytes escapes raw bytes into a wat data-segment string literal
func watBytes(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		sb.WriteString(fmt.Sprintf("\\%02x", c))
	}
	return sb.String()
}

// transferWatTo pays 10 raw units to the given address (the To
// address is embedded as a data segment)
func transferWatTo(to ngtypes.Address) string {
	return `
(module
  (import "coin" "transfer" (func $transfer (param i32 i64) (result i32)))
  (memory 1)
  (data (i32.const 0) "` + watBytes(to[:]) + `")
  (func (export "main")
    (drop (call $transfer (i32.const 0) (i64.const 10)))))
`
}

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

// putContract stores the contract slot and funds its address
func putContract(t *testing.T, txn *bbolt.Tx, acc *ngtypes.Contract, balance int64) {
	t.Helper()

	if err := setContract(txn, acc); err != nil {
		t.Fatal(err)
	}
	if err := setBalance(txn, acc.Owner, big.NewInt(balance)); err != nil {
		t.Fatal(err)
	}
}

func fakeTransactTx(to ngtypes.Address, value *big.Int) *ngtypes.FullTx {
	return ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
		to, value, big.NewInt(0), nil, nil)
}

// --- the tests ---

func TestVMLog(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(testAddr(0xaa), []byte(logWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
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
		acc := ngtypes.NewContract(testAddr(0xaa), []byte(kvWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}

		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		// reload from the db to prove the journal got flushed
		reloaded, err := getContract(txn, testAddr(0xaa))
		if err != nil {
			return err
		}

		if got := string(reloaded.Context.Get("key")); got != "val" {
			t.Fatalf("kv.set not applied, got %q", got)
		}
		if !reloaded.IsActive() {
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
		contractAcc := ngtypes.NewContract(testAddr(0xaa), []byte(transferWatTo(testAddr(0xbb))), nil)
		contractAcc.SetActive(true)
		putContract(t, txn, contractAcc, 100)

		vm, err := NewVM(txn, contractAcc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}

		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		if got := getBalance(txn, testAddr(0xaa)); got.Int64() != 90 {
			t.Fatalf("from balance = %d, want 90", got.Int64())
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
		acc := ngtypes.NewContract(testAddr(0xaa), []byte(burnWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 100)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}

		if err := vm.Run(VMEntryOnTx); err == nil {
			t.Fatal("infinite loop contract should be aborted by the toll station")
		}

		// the kv write before the loop must be gone
		reloaded, err := getContract(txn, testAddr(0xaa))
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

func TestActivateDeactivateFlow(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := ngtypes.NewAddress(priv)

	err = db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, []byte(logWat), nil)
		putContract(t, txn, acc, 100)

		// lock the account: the vm becomes active, editing gets frozen
		activateTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.ActivateTx, 1, ngtypes.Address{}, nil, big.NewInt(1), nil, nil)
		if err := activateTx.Signature(priv); err != nil {
			return err
		}
		if err := state.handleActivate(txn, activateTx, 1); err != nil {
			t.Fatalf("handleActivate: %v", err)
		}

		locked, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if !locked.IsActive() {
			t.Fatal("account should be locked")
		}
		if got := getBalance(txn, addr); got.Int64() != 99 {
			t.Fatalf("lock fee not charged, balance = %d", got.Int64())
		}

		// double lock must fail
		if err := state.handleActivate(txn, activateTx, 1); err == nil {
			t.Fatal("locking a locked account should fail")
		}

		// editing a locked account must fail
		commitTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.CommitTx, 1, ngtypes.Address{}, nil, big.NewInt(1), nil, nil)
		if err := commitTx.Signature(priv); err != nil {
			return err
		}
		if err := state.handleCommit(txn, commitTx); !errors.Is(err, ErrContractActive) {
			t.Fatalf("edit on locked account: got %v, want ErrContractActive", err)
		}

		// unlock reverts everything
		deactivateTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.DeactivateTx, 1, ngtypes.Address{}, nil, big.NewInt(1), nil, nil)
		if err := deactivateTx.Signature(priv); err != nil {
			return err
		}
		if err := state.handleDeactivate(txn, deactivateTx); err != nil {
			t.Fatalf("handleDeactivate: %v", err)
		}

		unlocked, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if unlocked.IsActive() {
			t.Fatal("account should be unlocked")
		}

		// double unlock must fail
		if err := state.handleDeactivate(txn, deactivateTx); err == nil {
			t.Fatal("unlocking an unlocked account should fail")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCommitFlow upgrades a contract with a minimal diff patch, and
// covers the namespace purchase: the FIRST edit must carry the
// one-time deploy fee on top of the tx fee
func TestCommitFlow(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := ngtypes.NewAddress(priv)

	baseWat := transferWatTo(testAddr(0xbb))
	newWat := strings.Replace(baseWat, "i64.const 10", "i64.const 25", 1)
	hunks := ngtypes.DiffHunks([]byte(baseWat), []byte(newWat))
	patchSize := 0
	for _, h := range hunks {
		patchSize += len(h.Del) + len(h.Ins)
	}
	if patchSize > 16 {
		t.Fatalf("small edit produced a big patch: %d bytes", patchSize)
	}

	deployExtra, err := ngtypes.NewCommitExtra(nil, []ngtypes.Hunk{{Pos: 0, Ins: []byte(baseWat)}}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	patchExtra, err := ngtypes.NewCommitExtra([]byte(baseWat), hunks).Encode()
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(txn *bbolt.Tx) error {
		deployTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.CommitTx, 1, ngtypes.Address{}, nil, big.NewInt(1), deployExtra, nil)
		if err := deployTx.Signature(priv); err != nil {
			return err
		}

		// an unfunded address cannot even pay the tx fee
		if err := checkCommit(txn, deployTx); err == nil {
			t.Fatal("deploy without covering the fee must fail")
		}

		// funded: the first commit opens the slot at plain fee cost
		if err := setBalance(txn, addr, big.NewInt(100)); err != nil {
			return err
		}
		if err := checkCommit(txn, deployTx); err != nil {
			t.Fatalf("checkCommit deploy: %v", err)
		}
		if err := state.handleCommit(txn, deployTx); err != nil {
			t.Fatalf("handleCommit deploy: %v", err)
		}
		if got := getBalance(txn, addr); got.Cmp(big.NewInt(99)) != 0 {
			t.Fatalf("deploy fee not burned, balance = %s", got)
		}

		// the second commit patches the existing slot
		commitTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.CommitTx, 1, ngtypes.Address{}, nil, big.NewInt(1), patchExtra, nil)
		if err := commitTx.Signature(priv); err != nil {
			return err
		}
		if err := checkCommit(txn, commitTx); err != nil {
			t.Fatalf("checkCommit patch: %v", err)
		}
		if err := state.handleCommit(txn, commitTx); err != nil {
			t.Fatalf("handleCommit patch: %v", err)
		}

		reloaded, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if string(reloaded.Source) != newWat {
			t.Fatalf("contract not patched:\n%s", reloaded.Source)
		}
		if got := getBalance(txn, addr); got.Cmp(big.NewInt(98)) != 0 {
			t.Fatalf("edit fee not charged, balance = %s", got)
		}

		// the patched text must still compile and lock
		activateTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.ActivateTx, 1, ngtypes.Address{}, nil, big.NewInt(1), nil, nil)
		if err := activateTx.Signature(priv); err != nil {
			return err
		}
		if err := state.handleActivate(txn, activateTx, 1); err != nil {
			t.Fatalf("handleActivate after edit: %v", err)
		}

		// a mismatching patch on the locked slot is refused as locked
		staleExtra, err := (&ngtypes.CommitExtra{Hunks: []ngtypes.Hunk{
			{Pos: 0, Del: []byte("XXX"), Ins: []byte("YYY")},
		}}).Encode()
		if err != nil {
			return err
		}
		staleTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.CommitTx, 1, ngtypes.Address{}, nil, big.NewInt(1), staleExtra, nil)
		if err := staleTx.Signature(priv); err != nil {
			return err
		}
		if err := checkCommit(txn, staleTx); !errors.Is(err, ErrContractActive) {
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

// leverageWatFor composes the dex module by its deployer address: it
// imports dex's algorithm and stores the computed result into its OWN
// kv state (delegate semantics)
func leverageWatFor(dex ngtypes.Address) string {
	return `
(module
  (import "contract/` + dex.String() + `" "double" (func $double (param i64) (result i64)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "num")
  (func (export "main")
    (i64.store8 (i32.const 8) (call $double (i64.const 21)))
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 8) (i32.const 1)))))
`
}

// TestContractModuleDeps covers the code-module dependency system:
// leverage imports dex by address, the reference pins dex until
// leverage releases it, and the linked execution runs dex's code on
// leverage's state
func TestContractModuleDeps(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	privDex, _ := ngtypes.GenerateKey()
	privLev, _ := ngtypes.GenerateKey()
	dexAddr := ngtypes.NewAddress(privDex)
	levAddr := ngtypes.NewAddress(privLev)

	err := db.Update(func(txn *bbolt.Tx) error {
		dex := ngtypes.NewContract(dexAddr, []byte(dexWat), nil)
		putContract(t, txn, dex, 100)
		lev := ngtypes.NewContract(levAddr, []byte(leverageWatFor(dexAddr)), nil)
		putContract(t, txn, lev, 100)

		activateTx := func(priv *ngtypes.PrivateKey) *ngtypes.FullTx {
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.ActivateTx, 1,
				ngtypes.Address{}, nil, big.NewInt(1), nil, nil)
			if err := tx.Signature(priv); err != nil {
				t.Fatal(err)
			}
			return tx
		}
		deactivateTx := func(priv *ngtypes.PrivateKey) *ngtypes.FullTx {
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.DeactivateTx, 1,
				ngtypes.Address{}, nil, big.NewInt(1), nil, nil)
			if err := tx.Signature(priv); err != nil {
				t.Fatal(err)
			}
			return tx
		}

		// locking leverage before its dependency is active must fail
		if err := state.handleActivate(txn, activateTx(privLev), 1); err == nil {
			t.Fatal("locking with an inactive dependency must fail")
		}

		// dex first, then leverage: the reference gets pinned
		if err := state.handleActivate(txn, activateTx(privDex), 1); err != nil {
			t.Fatalf("lock dex: %v", err)
		}
		if err := state.handleActivate(txn, activateTx(privLev), 1); err != nil {
			t.Fatalf("lock leverage: %v", err)
		}

		dexAcc, _ := getContract(txn, dexAddr)
		if getRefCount(dexAcc) != 1 {
			t.Fatalf("dex refcount = %d, want 1", getRefCount(dexAcc))
		}

		// the depended-on module can neither unlock nor be destroyed
		if err := state.handleDeactivate(txn, deactivateTx(privDex)); !errors.Is(err, ErrContractRefdBy) {
			t.Fatalf("unlock dex while referenced: got %v, want ErrContractRefdBy", err)
		}

		// linked execution: leverage's main calls dex's double and
		// writes 42 into leverage's own kv
		levAcc, _ := getContract(txn, levAddr)
		vm, err := NewVM(txn, levAcc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			t.Fatalf("NewVM with deps: %v", err)
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("linked run: %v", err)
		}

		levAcc, _ = getContract(txn, levAddr)
		if got := levAcc.Context.Get("num"); len(got) != 1 || got[0] != 42 {
			t.Fatalf("leverage kv num = %v, want [42]", got)
		}
		// delegate semantics: dex's own state stays untouched
		dexAcc, _ = getContract(txn, dexAddr)
		if got := dexAcc.Context.Get("num"); len(got) != 0 {
			t.Fatal("dex state must stay untouched")
		}

		// release: unlock leverage, then dex frees up
		if err := state.handleDeactivate(txn, deactivateTx(privLev)); err != nil {
			t.Fatalf("unlock leverage: %v", err)
		}
		dexAcc, _ = getContract(txn, dexAddr)
		if getRefCount(dexAcc) != 0 {
			t.Fatalf("dex refcount = %d after release, want 0", getRefCount(dexAcc))
		}
		if err := state.handleDeactivate(txn, deactivateTx(privDex)); err != nil {
			t.Fatalf("unlock dex after release: %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// tokenWat is a shared-ledger service (erc20-style): balances live in
// the TOKEN's own kv, keyed by the 32-byte address. The target address
// of transfer/mint_to crosses the service boundary through buf slot 1;
// account.get_caller (= msg.from) authorizes the debit
const tokenWat = `
(module
  (import "address" "get_caller" (func $caller (param i32) (result i32)))
  (import "env" "buf_get" (func $bget (param i32 i32) (result i32)))
  (import "kv" "get" (func $get (param i32 i32 i32) (result i32)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  ;; layout: 0..32 from-key (caller), 32..64 to-key (slot 1), 64..72 from-bal, 72..80 to-bal
  (func (export "transfer") (param $amount i64) (result i32)
    (drop (call $caller (i32.const 0)))
    (drop (call $bget (i32.const 1) (i32.const 32)))
    (i64.store (i32.const 64) (i64.const 0))
    (i64.store (i32.const 72) (i64.const 0))
    (drop (call $get (i32.const 0) (i32.const 32) (i32.const 64)))
    (drop (call $get (i32.const 32) (i32.const 32) (i32.const 72)))
    (if (i64.lt_u (i64.load (i32.const 64)) (local.get $amount))
      (then (return (i32.const 0))))
    (i64.store (i32.const 64) (i64.sub (i64.load (i32.const 64)) (local.get $amount)))
    (i64.store (i32.const 72) (i64.add (i64.load (i32.const 72)) (local.get $amount)))
    (drop (call $set (i32.const 0) (i32.const 32) (i32.const 64) (i32.const 8)))
    (drop (call $set (i32.const 32) (i32.const 32) (i32.const 72) (i32.const 8)))
    (i32.const 1))
  (func (export "mint_to") (param $amount i64)
    (drop (call $bget (i32.const 1) (i32.const 32)))
    (i64.store (i32.const 72) (i64.const 0))
    (drop (call $get (i32.const 32) (i32.const 32) (i32.const 72)))
    (i64.store (i32.const 72) (i64.add (i64.load (i32.const 72)) (local.get $amount)))
    (drop (call $set (i32.const 32) (i32.const 32) (i32.const 72) (i32.const 8)))))
`

// tokenUserWatFor consumes the token service: mints itself 100 units
// and sends 30 to dest — all bookkeeping happens inside the token
func tokenUserWatFor(token, self, dest ngtypes.Address) string {
	return `
(module
  (import "service/` + token.String() + `" "mint_to" (func $mint (param i64)))
  (import "service/` + token.String() + `" "transfer" (func $transfer (param i64) (result i32)))
  (import "env" "buf_set" (func $bset (param i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "` + watBytes(self[:]) + `")
  (data (i32.const 32) "` + watBytes(dest[:]) + `")
  (func (export "main")
    (drop (call $bset (i32.const 1) (i32.const 0) (i32.const 32)))
    (call $mint (i64.const 100))
    (drop (call $bset (i32.const 1) (i32.const 32) (i32.const 32)))
    (drop (call $transfer (i64.const 30)))))
`
}

// TestServiceToken covers own-state (service) semantics: the token's
// ledger lives in the token account's kv and is shared by all callers,
// with get_caller authorizing the debit
func TestServiceToken(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	privToken, _ := ngtypes.GenerateKey()
	privUser, _ := ngtypes.GenerateKey()
	tokenAddr := ngtypes.NewAddress(privToken)
	userAddr := ngtypes.NewAddress(privUser)
	dest := testAddr(0xcc)

	err := db.Update(func(txn *bbolt.Tx) error {
		token := ngtypes.NewContract(tokenAddr, []byte(tokenWat), nil)
		putContract(t, txn, token, 100)
		user := ngtypes.NewContract(userAddr, []byte(tokenUserWatFor(tokenAddr, userAddr, dest)), nil)
		putContract(t, txn, user, 100)

		lock := func(priv *ngtypes.PrivateKey) error {
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.ActivateTx, 1,
				ngtypes.Address{}, nil, big.NewInt(1), nil, nil)
			if err := tx.Signature(priv); err != nil {
				t.Fatal(err)
			}
			return state.handleActivate(txn, tx, 1)
		}

		if err := lock(privToken); err != nil {
			t.Fatalf("lock token: %v", err)
		}
		if err := lock(privUser); err != nil {
			t.Fatalf("lock token user: %v", err)
		}

		// the service dependency pins the token like a library dep does
		tokenAcc, _ := getContract(txn, tokenAddr)
		if getRefCount(tokenAcc) != 1 {
			t.Fatalf("token refcount = %d, want 1", getRefCount(tokenAcc))
		}

		// run the consumer: the ledger updates happen in the TOKEN's kv
		userAcc, _ := getContract(txn, userAddr)
		vm, err := NewVM(txn, userAcc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			t.Fatalf("NewVM with service dep: %v", err)
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("service run: %v", err)
		}

		tokenAcc, _ = getContract(txn, tokenAddr)
		balOf := func(addr ngtypes.Address) uint64 {
			raw := tokenAcc.Context.Get(string(addr[:]))
			if len(raw) != 8 {
				t.Fatalf("token ledger entry for %s missing: %v", addr, raw)
			}
			return binary.LittleEndian.Uint64(raw)
		}

		if got := balOf(userAddr); got != 70 {
			t.Fatalf("token bal[user] = %d, want 70", got)
		}
		if got := balOf(dest); got != 30 {
			t.Fatalf("token bal[dest] = %d, want 30", got)
		}

		// the consumer's own kv stays empty (reserved keys aside): the
		// ledger lived in the callee
		userAcc, _ = getContract(txn, userAddr)
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
		acc := ngtypes.NewContract(testAddr(0xaa), []byte(u256Wat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		reloaded, err := getContract(txn, testAddr(0xaa))
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
		acc := ngtypes.NewContract(owner, []byte(txCtxWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		// the tx pays 77 to the contract's address (msg.value)
		tx := fakeTransactTx(owner, big.NewInt(77))

		vm, err := NewVM(txn, acc, tx, 1755264000) // block timestamp
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		reloaded, err := getContract(txn, owner)
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
		acc := ngtypes.NewContract(testAddr(0xaa), []byte(kvScanWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		reloaded, err := getContract(txn, testAddr(0xaa))
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

// bigCallerWatFor deposits (2^128 - 1) twice: the second call must
// carry the full 256-bit total back across the boundary
func bigCallerWatFor(vault ngtypes.Address) string {
	return `
(module
  (import "env" "buf_set" (func $bset (param i32 i32 i32) (result i32)))
  (import "env" "buf_get" (func $bget (param i32 i32) (result i32)))
  (import "kv" "set" (func $kvset (param i32 i32 i32 i32) (result i32)))
  (import "service/` + vault.String() + `" "deposit_big" (func $deposit))
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

	vaultAddr := testAddr(0xaa)
	callerAddr := testAddr(0xdd)

	err := db.Update(func(txn *bbolt.Tx) error {
		vault := ngtypes.NewContract(vaultAddr, []byte(bigVaultWat), nil)
		vault.SetActive(true)
		putContract(t, txn, vault, 0)

		caller := ngtypes.NewContract(callerAddr, []byte(bigCallerWatFor(vaultAddr)), nil)
		caller.SetActive(true)
		putContract(t, txn, caller, 0)

		vm, err := NewVM(txn, caller, fakeTransactTx(ngtypes.Address{}, nil), 1)
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

		callerAcc, _ := getContract(txn, callerAddr)
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
		vaultAcc, _ := getContract(txn, vaultAddr)
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

	emitAddr := testAddr(0xaa)
	badAddr := testAddr(0xbb)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(emitAddr, []byte(emitWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		tx := fakeTransactTx(ngtypes.Address{}, nil)
		state.runContract(txn, emitAddr, tx, VMEntryOnTx, 1)

		runs, err := GetTxRuns(txn, tx.GetHash())
		if err != nil {
			return err
		}
		if len(runs) != 1 {
			t.Fatalf("runs = %d, want 1", len(runs))
		}
		run := runs[0]
		if !run.Ok || string(run.Contract) != string(emitAddr[:]) || run.Entry != VMEntryOnTx {
			t.Fatalf("run = %+v", run)
		}
		if run.GasUsed == 0 {
			t.Fatal("run must report gas")
		}
		if len(run.Events) != 2 {
			t.Fatalf("events = %d, want 2", len(run.Events))
		}
		if run.Events[0].Topic != "transfer" || string(run.Events[0].Data) != "data1" ||
			string(run.Events[0].Contract) != string(emitAddr[:]) {
			t.Fatalf("event[0] = %+v", run.Events[0])
		}
		if run.Events[1].Topic != "mint" || len(run.Events[1].Data) != 0 {
			t.Fatalf("event[1] = %+v", run.Events[1])
		}

		// a failing contract records the failure and drops its events
		bad := ngtypes.NewContract(badAddr, []byte(burnWat), nil)
		bad.SetActive(true)
		putContract(t, txn, bad, 0)

		badTx := fakeTransactTx(badAddr, big.NewInt(0))
		state.runContract(txn, badAddr, badTx, VMEntryOnTx, 1)

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
			acc := ngtypes.NewContract(testAddr(0xaa), []byte(wat), nil)
			acc.SetActive(true)
			putContract(t, txn, acc, 100)

			vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
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
		acc := ngtypes.NewContract(testAddr(0xaa), []byte(kvWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
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
		reloaded, err := getContract(txn, testAddr(0xaa))
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

// TestDestroyRules: a slot cannot be destroyed while its contract is
// active (locked) — downstream contracts may depend on it; after
// unlock, destroy removes the slot (with its Context) entirely
func TestDestroyRules(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := ngtypes.NewAddress(priv)

	err = db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, []byte(logWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 100)

		destroyTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.DestroyTx, 1, ngtypes.Address{}, nil, big.NewInt(1), nil, nil)
		if err := destroyTx.Signature(priv); err != nil {
			return err
		}

		// locked: refused
		if err := state.handleDestroy(txn, destroyTx); err == nil {
			t.Fatal("destroying a locked account must fail")
		}

		// unlocked: destroy goes through and the slot is gone
		acc.SetActive(false)
		if err := setContract(txn, acc); err != nil {
			return err
		}
		if err := state.handleDestroy(txn, destroyTx); err != nil {
			t.Fatalf("destroy after unlocking: %v", err)
		}
		if _, err := getContract(txn, addr); err == nil {
			t.Fatal("the slot (and its context) must be removed")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestActivateRejectsBrokenContract(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := ngtypes.NewAddress(priv)

	err = db.Update(func(txn *bbolt.Tx) error {
		// a half-edited contract text must not be lockable
		acc := ngtypes.NewContract(addr, []byte(`(module (func (export "main")`), nil)
		putContract(t, txn, acc, 100)

		activateTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.ActivateTx, 1, ngtypes.Address{}, nil, big.NewInt(1), nil, nil)
		if err := activateTx.Signature(priv); err != nil {
			return err
		}

		if err := checkActivate(txn, activateTx); err == nil {
			t.Fatal("checkActivate should reject a non-compiling contract")
		}
		if err := state.handleActivate(txn, activateTx, 1); err == nil {
			t.Fatal("handleActivate should reject a non-compiling contract")
		}

		reloaded, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if reloaded.IsActive() {
			t.Fatal("account must stay unlocked after a failed lock")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// multiEntryWat exposes two callable methods besides main: each writes
// which entry ran, ping also stores the args it received
const multiEntryWat = `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (import "tx" "get_extra_size" (func $alen (result i32)))
  (import "tx" "get_extra" (func $args (param i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "hitmainpingargs")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 4)))
    (drop (call $set (i32.const 11) (i32.const 4) (i32.const 64) (call $args (i32.const 64)))))
  (func (export "ping")
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 7) (i32.const 4)))
    (drop (call $set (i32.const 11) (i32.const 4) (i32.const 64) (call $args (i32.const 64))))))
`

// TestCallSelector: the eth-style 4-byte selector routes a transact to
// a named export; unknown selectors fall back to main which sees the
// whole extra (eth fallback semantics)
func TestCallSelector(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	addr := testAddr(0xaa)

	callWith := func(extra []byte) *ngtypes.Contract {
		var reloaded *ngtypes.Contract
		err := db.Update(func(txn *bbolt.Tx) error {
			acc := ngtypes.NewContract(addr, []byte(multiEntryWat), nil)
			acc.SetActive(true)
			putContract(t, txn, acc, 0)

			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
				addr, big.NewInt(0), big.NewInt(0), extra, nil)
			state.runContract(txn, addr, tx, VMEntryOnTx, 1)

			var err error
			reloaded, err = getContract(txn, addr)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return reloaded
	}

	// selector picks ping; args are the bytes after the selector
	acc := callWith(ngtypes.EncodeCallData("ping", []byte("xy")))
	if got := string(acc.Context.Get("hit")); got != "ping" {
		t.Fatalf("hit = %q, want ping", got)
	}
	if got := string(acc.Context.Get("args")); got != "xy" {
		t.Fatalf("args = %q, want xy", got)
	}

	// explicit main selector also routes, args stripped of the selector
	acc = callWith(ngtypes.EncodeCallData("main", []byte("ab")))
	if got := string(acc.Context.Get("hit")); got != "main" {
		t.Fatalf("hit = %q, want main", got)
	}
	if got := string(acc.Context.Get("args")); got != "ab" {
		t.Fatalf("args = %q, want ab", got)
	}

	// unknown selector: fallback to main, which sees the WHOLE extra
	raw := append(ngtypes.CallSelector("nope"), []byte("zz")...)
	acc = callWith(raw)
	if got := string(acc.Context.Get("hit")); got != "main" {
		t.Fatalf("hit = %q, want main (fallback)", got)
	}
	if got := string(acc.Context.Get("args")); got != string(raw) {
		t.Fatalf("fallback args = %x, want the whole extra %x", got, raw)
	}

	// empty extra: plain main, no args
	acc = callWith(nil)
	if got := string(acc.Context.Get("hit")); got != "main" {
		t.Fatalf("hit = %q, want main", got)
	}
	if got := acc.Context.Get("args"); len(got) != 0 {
		t.Fatalf("args = %x, want empty", got)
	}
}

// TestCompactEnvelope: after an address's first full-envelope tx
// registers its key, later txs may drop the 897-byte public key; an
// unregistered address's compact tx is refused
func TestCompactEnvelope(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)
	var dest ngtypes.Address
	dest[0] = 0xd1

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, addr, big.NewInt(100)); err != nil {
			return err
		}

		// a compact tx before ANY full tx must be refused: no key on chain
		early := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
			dest, big.NewInt(1), nil, nil, nil)
		if err := early.SignatureCompact(priv); err != nil {
			return err
		}
		if len(early.Sign) != 2+ngtypes.AddressSize+ngtypes.SigSize(priv.Scheme) {
			t.Fatalf("compact envelope size = %d", len(early.Sign))
		}
		if err := checkTransaction(txn, early); err == nil {
			t.Fatal("compact envelope without a registered key must fail")
		}

		// the first FULL tx registers the key
		full := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
			dest, big.NewInt(1), nil, nil, nil)
		if err := full.Signature(priv); err != nil {
			return err
		}
		if len(full.Sign) != 2+ngtypes.PubKeySize(priv.Scheme)+ngtypes.SigSize(priv.Scheme) {
			t.Fatalf("full envelope size = %d", len(full.Sign))
		}
		if err := state.handleTransaction(txn, full, 1); err != nil {
			t.Fatalf("full tx: %v", err)
		}

		// now the compact form spends fine
		compact := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
			dest, big.NewInt(2), nil, nil, nil)
		if err := compact.SignatureCompact(priv); err != nil {
			return err
		}
		if err := checkTransaction(txn, compact); err != nil {
			t.Fatalf("checkTransaction compact: %v", err)
		}
		if err := state.handleTransaction(txn, compact, 1); err != nil {
			t.Fatalf("compact tx: %v", err)
		}
		if got := getBalance(txn, dest); got.Int64() != 3 {
			t.Fatalf("dest balance = %d, want 3", got.Int64())
		}

		// a compact envelope claiming a FOREIGN registered address must
		// fail: the signature does not verify under that key
		other, _ := ngtypes.GenerateKey()
		forged := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
			dest, big.NewInt(1), nil, nil, nil)
		if err := forged.SignatureCompact(other); err != nil {
			return err
		}
		copy(forged.Sign[:ngtypes.AddressSize], addr[:]) // pose as addr
		if err := checkTransaction(txn, forged); err == nil {
			t.Fatal("a forged compact envelope must fail")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
