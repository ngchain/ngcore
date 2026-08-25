package ngstate

import (
	"strings"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestLoadContractWasmRejects covers the loader's refusal paths
func TestLoadContractWasmRejects(t *testing.T) {
	if _, err := LoadContractWasm(nil); err == nil {
		t.Fatal("empty source accepted")
	}
	if _, err := LoadContractWasm([]byte("no")); err == nil {
		t.Fatal("short source accepted")
	}
	if _, err := LoadContractWasm([]byte("not-wasm-at-all")); err == nil {
		t.Fatal("magicless source accepted")
	}
	if _, err := LoadContractWasm([]byte{0x00, 0x61, 0x73, 0x6d, 0xde, 0xad}); err == nil {
		t.Fatal("magic + garbage accepted")
	}
	bin, err := LoadContractWasm(mustWat(logWat))
	if err != nil || bin == nil {
		t.Fatalf("valid module refused: %v", err)
	}
}

// TestLimitTollAndGasUsed pins the block-cap pre-burn arithmetic
func TestLimitTollAndGasUsed(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(testAddr(0xa1), mustWat(logWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		// a budget at (or above) the default is a no-op
		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		vm.LimitToll(vmMaxToll)
		if vm.tollPreburn != 0 {
			t.Fatal("a full budget must not pre-burn")
		}
		if got := vm.GasUsed(); got != 0 {
			t.Fatalf("gas before running = %d", got)
		}

		// a shrunken budget pre-burns the difference and GasUsed reports
		// only this run's own consumption
		vm2, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		vm2.LimitToll(vmMaxToll / 2)
		if vm2.tollPreburn != vmMaxToll/2 {
			t.Fatalf("preburn = %d", vm2.tollPreburn)
		}
		if got := vm2.GasUsed(); got != 0 {
			t.Fatalf("gas right after the pre-burn = %d", got)
		}
		if err := vm2.Run(VMEntryOnTx); err != nil {
			return err
		}
		if got := vm2.GasUsed(); got == 0 || got >= vmMaxToll/2 {
			t.Fatalf("own gas = %d, want within (0, %d)", got, int64(vmMaxToll/2))
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestHostChargeOverflow: a budget too small for a host operation's
// surcharge aborts the call deterministically (the charge panic is
// recovered into a run error) and the journal is dropped
func TestHostChargeOverflow(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(testAddr(0xa2), mustWat(kvWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		// enough for the instructions, not for the kv.set surcharge
		vm.LimitToll(500)

		if err := vm.Run(VMEntryOnTx); err == nil {
			t.Fatal("the host surcharge must overflow the tiny budget")
		}

		reloaded, err := getContract(txn, testAddr(0xa2))
		if err != nil {
			return err
		}
		if got := reloaded.Context.Get("key"); len(got) != 0 {
			t.Fatalf("journal leaked through the gas abort: %q", got)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestEntryForRules pins the entry resolution outside the dispatch
// paths already covered end-to-end
func TestEntryForRules(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(testAddr(0xa3), mustWat(multiEntryWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		// a non-default entry (init) never consults the calldata
		tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
			testAddr(0xa3), nil, nil, ngtypes.EncodeCallData("ping", nil), nil)
		vm, err := NewVM(txn, acc, tx, 1)
		if err != nil {
			return err
		}
		if got := vm.EntryFor(VMEntryOnActivate); got != VMEntryOnActivate {
			t.Fatalf("EntryFor(init) = %q", got)
		}
		// the default entry with a method-carrying calldata dispatches
		if got := vm.EntryFor(VMEntryOnTx); got != "ping" {
			t.Fatalf("EntryFor(main) = %q", got)
		}

		// an empty extra always stays on the default entry
		vm2, err := NewVM(txn, acc, fakeTransactTx(testAddr(0xa3), nil), 1)
		if err != nil {
			return err
		}
		if got := vm2.EntryFor(VMEntryOnTx); got != VMEntryOnTx {
			t.Fatalf("EntryFor(main, no extra) = %q", got)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRunInstantiateFailure: a module importing an unknown host module
// fails at instantiation, not with a panic
func TestRunInstantiateFailure(t *testing.T) {
	db := newTestDB(t)

	watSrc := `
(module
  (import "nosuchhost" "f" (func $f))
  (func (export "ng:main") (call $f)))
`

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(testAddr(0xa4), mustWat(watSrc), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		err = vm.Run(VMEntryOnTx)
		if err == nil || !strings.Contains(err.Error(), "instantiate") {
			t.Fatalf("unlinkable module: got %v", err)
		}

		// DryRun degrades the same way
		vm2, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		if _, err := vm2.DryRun(VMEntryOnTx); err == nil {
			t.Fatal("DryRun must fail on an unlinkable module")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestVMFrameHelpers pins the call-frame identity helpers on a fresh vm
func TestVMFrameHelpers(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		owner := testAddr(0xa5)
		acc := ngtypes.NewContract(owner, mustWat(logWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		if got := vm.currentAddress(); !got.Equals(owner) {
			t.Fatalf("currentAddress = %s", got)
		}
		if got := vm.callerAddress(); got != (ngtypes.Address{}) {
			t.Fatalf("outermost callerAddress = %s", got)
		}
		if !vm.onStack(owner) || vm.onStack(testAddr(0xa6)) {
			t.Fatal("onStack misreports the frame stack")
		}
		if vm.Events() != nil {
			t.Fatal("a fresh vm must have no events")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestIsExportMissing distinguishes the missing-export condition from
// other failures
func TestIsExportMissing(t *testing.T) {
	db := newTestDB(t)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(testAddr(0xa7), mustWat(logWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		err = vm.Run("no_such_export")
		if err == nil || !IsExportMissing(err) {
			t.Fatalf("missing export: got %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
