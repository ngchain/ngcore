package ngstate

import (
	"errors"
	"math/big"
	"reflect"
	"testing"

	"github.com/c0mm4nd/rlp"
	"github.com/c0mm4nd/wasman/types"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestServiceRawConversions pins the wasm<->Go value marshalling of the
// service wrapper layer, including the float refusals
func TestServiceRawConversions(t *testing.T) {
	if goTypeOf(types.ValueTypeI32) != reflect.TypeOf(uint32(0)) {
		t.Fatal("i32 must map to uint32")
	}
	if goTypeOf(types.ValueTypeI64) != reflect.TypeOf(uint64(0)) {
		t.Fatal("i64 must map to uint64")
	}
	if goTypeOf(types.ValueTypeF32) != reflect.TypeOf(float32(0)) {
		t.Fatal("f32 must map to float32")
	}
	if goTypeOf(types.ValueTypeF64) != reflect.TypeOf(float64(0)) {
		t.Fatal("f64 must map to float64")
	}

	if got := toRaw(reflect.ValueOf(uint32(7))); got != 7 {
		t.Fatalf("toRaw(u32) = %d", got)
	}
	if got := toRaw(reflect.ValueOf(uint64(9))); got != 9 {
		t.Fatalf("toRaw(u64) = %d", got)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("toRaw(float) must panic")
			}
		}()
		_ = toRaw(reflect.ValueOf(float32(1)))
	}()

	if got := fromRaw(7, reflect.TypeOf(uint32(0))); got.Interface() != uint32(7) {
		t.Fatalf("fromRaw(u32) = %v", got)
	}
	if got := fromRaw(9, reflect.TypeOf(uint64(0))); got.Interface() != uint64(9) {
		t.Fatalf("fromRaw(u64) = %v", got)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("fromRaw(float) must panic")
			}
		}()
		_ = fromRaw(1, reflect.TypeOf(float64(0)))
	}()
}

// TestResolveDynEntry pins the dynamic-call dispatch rule: named
// zero-arg exports run, everything else falls back to main with the
// args preserved
func TestResolveDynEntry(t *testing.T) {
	multi, err := templateFor(mustWat(multiEntryWat)) // exports main + ping
	if err != nil {
		t.Fatal(err)
	}

	entry, args := resolveDynEntry(multi, ngtypes.EncodeCallData("ping", []byte("xy")))
	if entry != "ping" || string(args) != "xy" {
		t.Fatalf("ping dispatch = %q/%q", entry, args)
	}

	// an unknown method falls back to main, keeping the args
	entry, args = resolveDynEntry(multi, ngtypes.EncodeCallData("nope", []byte("zz")))
	if entry != VMEntryOnTx || string(args) != "zz" {
		t.Fatalf("unknown method = %q/%q", entry, args)
	}

	// the reserved init entry is not callable
	entry, _ = resolveDynEntry(multi, ngtypes.EncodeCallData("ng:init", nil))
	if entry != VMEntryOnTx {
		t.Fatalf("init dispatch = %q", entry)
	}

	// a non-CallData payload: main sees the whole calldata
	raw := []byte("not-rlp-calldata")
	entry, args = resolveDynEntry(multi, raw)
	if entry != VMEntryOnTx || string(args) != string(raw) {
		t.Fatalf("raw calldata = %q/%q", entry, args)
	}

	// a method with parameters is not dispatchable
	dex, err := templateFor(mustWat(dexWat)) // double(i64)
	if err != nil {
		t.Fatal(err)
	}
	entry, _ = resolveDynEntry(dex, ngtypes.EncodeCallData("double", nil))
	if entry != VMEntryOnTx {
		t.Fatalf("param method dispatch = %q", entry)
	}
}

// TestExportFuncSig covers the export introspection: functions resolve,
// non-function exports and re-exported imports do not
func TestExportFuncSig(t *testing.T) {
	watSrc := `
(module
  (import "kv" "del" (func $d (param i32 i32) (result i32)))
  (memory 1)
  (export "mem" (memory 0))
  (export "redel" (func $d))
  (func (export "f") (param i64) (result i64) (local.get 0)))
`
	module, err := templateFor(mustWat(watSrc))
	if err != nil {
		t.Fatal(err)
	}

	sig, ok := exportFuncSig(module, "f")
	if !ok || len(sig.InputTypes) != 1 || len(sig.ReturnTypes) != 1 {
		t.Fatalf("f sig = %+v ok=%v", sig, ok)
	}
	if _, ok := exportFuncSig(module, "nosuch"); ok {
		t.Fatal("a missing export must not resolve")
	}
	if _, ok := exportFuncSig(module, "mem"); ok {
		t.Fatal("a memory export must not resolve as a function")
	}
	if _, ok := exportFuncSig(module, "redel"); ok {
		t.Fatal("a re-exported import must not resolve as a service entry")
	}
}

// TestServiceDepFailures covers the service linking error paths: a dep
// with broken code, an inactive dep, an unknown dep
func TestServiceDepFailures(t *testing.T) {
	db := newTestDB(t)

	depAddr := testAddr(0x71)
	callerAddr := testAddr(0x72)

	callerWat := `
(module
  (import "` + depAddr.String() + `" "f" (func $f))
  (func (export "ng:main") (call $f)))
`

	err := db.Update(func(txn *bbolt.Tx) error {
		caller := ngtypes.NewContract(callerAddr, mustWat(callerWat), nil)
		caller.SetActive(true)
		putContract(t, txn, caller, 0)

		// no dep contract at all
		if _, err := NewVM(txn, caller, fakeTransactTx(ngtypes.Address{}, nil), 1); err == nil {
			t.Fatal("linking an unknown dep must fail")
		}

		// the dep exists but is inactive
		dep := ngtypes.NewContract(depAddr, mustWat(`(module (func (export "f")))`), nil)
		putContract(t, txn, dep, 0)
		if _, err := NewVM(txn, caller, fakeTransactTx(ngtypes.Address{}, nil), 1); !errors.Is(err, ErrDepNotActive) {
			t.Fatalf("inactive dep: got %v", err)
		}

		// the dep is active but its stored code is garbage (forced in,
		// bypassing activation): linking must refuse it
		dep.Source = []byte{0x00, 0x61, 0x73, 0x6d, 0xff, 0xff}
		dep.SetActive(true)
		if err := setContract(txn, nil, dep); err != nil {
			return err
		}
		if _, err := NewVM(txn, caller, fakeTransactTx(ngtypes.Address{}, nil), 1); err == nil {
			t.Fatal("a non-compiling dep must fail the link")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestDepHelpers covers the dependency bookkeeping primitives
func TestDepHelpers(t *testing.T) {
	// bad bs58 identifiers are refused
	if _, err := resolveDepAddr("!!not-bs58!!"); !errors.Is(err, ErrDepInvalidImport) {
		t.Fatalf("bad dep addr: got %v", err)
	}

	// empty source has no deps
	deps, err := extractContractDeps(nil)
	if err != nil || deps != nil {
		t.Fatalf("empty source deps = %v, %v", deps, err)
	}

	// garbage source errors
	if _, err := extractContractDeps([]byte("garbage")); err == nil {
		t.Fatal("garbage source must error")
	}

	// a real import resolves to its address (deduplicated)
	dep := testAddr(0x81)
	watSrc := `
(module
  (import "` + dep.String() + `" "a" (func $a))
  (import "` + dep.String() + `" "b" (func $b))
  (func (export "ng:main")))
`
	deps, err = extractContractDeps(mustWat(watSrc))
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || !deps[0].Equals(dep) {
		t.Fatalf("deps = %v", deps)
	}

	// the per-contract import cap
	manyImports := "(module "
	for i := 0; i < maxDepsPerContract+1; i++ {
		a := testAddr(byte(i + 1))
		manyImports += `(import "` + a.String() + `" "f" (func))`
	}
	manyImports += `(func (export "ng:main")))`
	if _, err := extractContractDeps(mustWat(manyImports)); !errors.Is(err, ErrDepLimit) {
		t.Fatalf("dep cap: got %v", err)
	}

	// the persisted dep ledger round trip
	acc := ngtypes.NewContract(testAddr(0x82), nil, nil)
	if err := setContractDeps(acc, []ngtypes.Address{dep}); err != nil {
		t.Fatal(err)
	}
	back, err := getContractDeps(acc)
	if err != nil || len(back) != 1 || !back[0].Equals(dep) {
		t.Fatalf("dep ledger round trip = %v, %v", back, err)
	}
	if err := setContractDeps(acc, nil); err != nil {
		t.Fatal(err)
	}
	if back, _ := getContractDeps(acc); back != nil {
		t.Fatal("clearing the ledger must drop the key")
	}

	// corrupted ledgers error instead of returning junk
	acc.Context.Set(contextKeyDeps, []byte{0xff, 0xfe})
	if _, err := getContractDeps(acc); err == nil {
		t.Fatal("broken rlp ledger must error")
	}
	short, err := rlp.EncodeToBytes([][]byte{{1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	acc.Context.Set(contextKeyDeps, short)
	if _, err := getContractDeps(acc); !errors.Is(err, ErrDepInvalidImport) {
		t.Fatalf("short dep entry: got %v", err)
	}

	// refcount encoding: junk reads as zero, zero clears the key
	acc.Context.Set(contextKeyRefs, []byte{1, 2})
	if getRefCount(acc) != 0 {
		t.Fatal("junk refcount must read as 0")
	}
	setRefCount(acc, 3)
	if getRefCount(acc) != 3 {
		t.Fatal("refcount round trip failed")
	}
	setRefCount(acc, 0)
	if len(acc.Context.Get(contextKeyRefs)) != 0 {
		t.Fatal("zero refcount must clear the key")
	}
}

// TestJournalPrimitives covers the journal's read-your-writes overlay
// and its refusal paths
func TestJournalPrimitives(t *testing.T) {
	db := newTestDB(t)

	self := ngtypes.NewContract(testAddr(0x91), nil, nil)
	other := testAddr(0x92)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, self.Owner, big.NewInt(50)); err != nil {
			return err
		}

		j := newVMJournal(self)

		// negative and over-balance transfers are refused
		if err := j.transfer(txn, self.Owner, other, big.NewInt(-1)); !errors.Is(err, ngtypes.ErrTxValueInvalid) {
			t.Fatalf("negative transfer: got %v", err)
		}
		if err := j.transfer(txn, self.Owner, other, big.NewInt(51)); !errors.Is(err, ErrTxrBalanceInsufficient) {
			t.Fatalf("over-balance transfer: got %v", err)
		}

		// a valid transfer is visible through the overlay only
		if err := j.transfer(txn, self.Owner, other, big.NewInt(20)); err != nil {
			t.Fatal(err)
		}
		if got := j.balanceOf(txn, self.Owner); got.Int64() != 30 {
			t.Fatalf("journaled balance = %s", got)
		}
		if got := j.balanceOf(txn, other); got.Int64() != 20 {
			t.Fatalf("journaled dest balance = %s", got)
		}
		if got := getBalance(txn, other); got.Sign() != 0 {
			t.Fatal("the transfer leaked before flush")
		}

		// the context of an address with no slot cannot be loaded
		if _, err := j.contextOf(txn, testAddr(0x93)); err == nil {
			t.Fatal("contextOf a slotless address must error")
		}
		// the self context is the pre-cloned working copy
		ctx, err := j.contextOf(txn, self.Owner)
		if err != nil {
			return err
		}
		ctx.Set("k", []byte("v"))

		if err := j.flush(txn, nil); err != nil {
			return err
		}
		if got := getBalance(txn, other); got.Int64() != 20 {
			t.Fatalf("flushed balance = %s", got)
		}
		reloaded, err := getContract(txn, self.Owner)
		if err != nil {
			return err
		}
		if string(reloaded.Context.Get("k")) != "v" {
			t.Fatal("flushed context lost")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
