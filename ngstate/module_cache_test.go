package ngstate

import (
	"math/big"
	"sync"
	"testing"

	"github.com/c0mm4nd/wasman/config"
	"github.com/c0mm4nd/wasman/tollstation"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// rentFund is a generous per-contract balance covering the storage deposit
// its kv writes lock (active from genesis): kvWat stores 6 bytes -> 6e12, and
// this is far above any single test's footprint. int64 holds it (< 9.2e18).
const rentFund int64 = 1_000_000_000_000_000_000 // 1 NG

// TestModuleCache pins the wasm template cache's contract: one decoded
// template per code hash, per-call configs bound on shallow copies only,
// malformed sources never cached, and the shared template safe under
// concurrent per-call instantiation (run with -race).
func TestModuleCache(t *testing.T) {
	code := mustWat(kvWat)

	// same source -> the SAME cached template pointer
	t1, err := templateFor(code)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := templateFor(code)
	if err != nil {
		t.Fatal(err)
	}
	if t1 != t2 {
		t.Fatal("templateFor did not memoize: two pointers for one source")
	}

	// per-call copies carry their own config; the template stays plain
	depth := vmCallDepth
	m1, err := loadModule(code, config.ModuleConfig{
		Recover:        true,
		CallDepthLimit: &depth,
		EnableJIT:      true,
		TollStation:    tollstation.NewSimpleTollStation(vmMaxToll),
	})
	if err != nil {
		t.Fatal(err)
	}
	if m1 == t1 {
		t.Fatal("loadModule returned the shared template, not a copy")
	}
	if t1.ModuleConfig.TollStation != nil || t1.ModuleConfig.EnableJIT {
		t.Fatal("per-call config leaked into the cached template")
	}

	// malformed source: error every time, never cached
	if _, err := templateFor([]byte("\x00asm garbage")); err == nil {
		t.Fatal("malformed source accepted")
	}
	if _, err := templateFor([]byte("\x00asm garbage")); err == nil {
		t.Fatal("malformed source cached as valid on second sight")
	}
}

// TestModuleCacheConcurrent runs full VM calls for the SAME contract from
// many goroutines: every call instantiates from the one cached template.
// Guards the cache's core safety claim — instantiation must never mutate
// the shared decoded sections (meaningful under -race).
func TestModuleCacheConcurrent(t *testing.T) {
	code := mustWat(kvWat)
	const workers, rounds = 8, 10

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			db := newTestDB(t)
			addr := testAddr(byte(0x40 + w))
			err := db.Update(func(txn *bbolt.Tx) error {
				acc := ngtypes.NewContract(addr, code, nil)
				acc.SetActive(true)
				putContract(t, txn, acc, rentFund) // fund the kv storage deposit
				for i := 0; i < rounds; i++ {
					vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
					if err != nil {
						return err
					}
					if err := vm.Run(VMEntryOnTx); err != nil {
						return err
					}
				}
				reloaded, err := getContract(txn, addr)
				if err != nil {
					return err
				}
				if got := string(reloaded.Context.Get("key")); got != "val" {
					t.Errorf("worker %d: kv.set lost under concurrency, got %q", w, got)
				}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}(w)
	}
	wg.Wait()
}

// TestGasGolden locks the exact toll of a fixed contract call. Gas is
// consensus state: ANY engine or pricing change that shifts it must fail
// here loudly instead of splitting the network silently.
func TestGasGolden(t *testing.T) {
	db := newTestDB(t)
	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(testAddr(0x77), mustWat(kvWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, rentFund) // fund the kv storage deposit
		var got [2]uint64
		for i := range got {
			vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
			if err != nil {
				return err
			}
			if err := vm.Run(VMEntryOnTx); err != nil {
				return err
			}
			got[i] = vm.GasUsed()
		}
		if got[0] != got[1] {
			t.Fatalf("gas not deterministic across calls: %d vs %d", got[0], got[1])
		}
		const golden = 1066 // kvWat main under the metered JIT (== interpreter)
		if got[0] != golden {
			t.Fatalf("gas changed: %d, golden %d — consensus-breaking engine change", got[0], golden)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestServiceTrapRollback: a service CALLEE trapping (unreachable) must
// abort the whole call — the caller's own journal writes from before the
// service call are dropped, nothing lands in either contract's kv, and gas
// is still accounted. Exercises the trap-unwinding path across the
// caller-instance/callee-instance boundary under the metered JIT.
func TestServiceTrapRollback(t *testing.T) {
	db := newTestDB(t)

	privCallee, _ := ngtypes.GenerateKey()
	privCaller, _ := ngtypes.GenerateKey()
	calleeAddr := ngtypes.NewAddress(privCallee)
	callerAddr := ngtypes.NewAddress(privCaller)

	calleeWat := `(module
		(func (export "boom") (unreachable)))`
	callerWat := `(module
		(import "` + calleeAddr.String() + `" "boom" (func $boom))
		(import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
		(memory 1)
		(data (i32.const 0) "prex")
		(func (export "ng:main")
			;; a write BEFORE the failing service call: must not survive
			(drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 1)))
			(call $boom)))`

	err := db.Update(func(txn *bbolt.Tx) error {
		callee := ngtypes.NewContract(calleeAddr, mustWat(calleeWat), nil)
		callee.SetActive(true)
		putContract(t, txn, callee, 100)
		caller := ngtypes.NewContract(callerAddr, mustWat(callerWat), nil)
		caller.SetActive(true)
		putContract(t, txn, caller, rentFund) // fund the pre-trap kv write's deposit

		// snapshot both contracts' key sets before the failing run
		preKeys := func(a ngtypes.Address) int {
			acc, _ := getContract(txn, a)
			return len(acc.Context.Keys)
		}
		callerPre, calleePre := preKeys(callerAddr), preKeys(calleeAddr)

		callerAcc, _ := getContract(txn, callerAddr)
		vm, err := NewVM(txn, callerAcc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := vm.Run(VMEntryOnTx); err == nil {
			t.Fatal("callee trap did not abort the caller")
		}
		if vm.GasUsed() == 0 {
			t.Fatal("failed call burned no gas")
		}

		// nothing may have leaked into either contract's kv
		callerAcc, _ = getContract(txn, callerAddr)
		if got := callerAcc.Context.Get("pre"); got != nil {
			t.Fatalf("caller journal write survived the trap: %q", got)
		}
		if got := preKeys(callerAddr); got != callerPre {
			t.Fatalf("caller kv keys changed by the aborted call: %d -> %d", callerPre, got)
		}
		if got := preKeys(calleeAddr); got != calleePre {
			t.Fatalf("callee kv keys changed by the aborted call: %d -> %d", calleePre, got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestReentryGuard: a contract dynamically calling back into an address
// already on the call stack (here: itself) must abort with the reentry
// guard — reentrancy is the classic theft primitive and must be dead on
// arrival, with the whole call rolled back.
func TestReentryGuard(t *testing.T) {
	db := newTestDB(t)
	self := testAddr(0x5e)

	// main: contract.call(own address) — the guard must kill it
	watSrc := `
(module
  (import "contract" "call" (func $call (param i32 i32 i32) (result i32)))
  (import "address" "get_host" (func $host (param i32) (result i32)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "prex")
  ;; RLP CallData{method:"ping", args:[]} = c6 84 'ping' 80
  (data (i32.const 600) "\c6\84\70\69\6e\67\80")
  (func (export "ping"))
  (func (export "ng:main")
    ;; a journal write BEFORE the illegal call: must not survive
    (drop (call $set (i32.const 0) (i32.const 3) (i32.const 3) (i32.const 1)))
    (drop (call $host (i32.const 512)))
    (drop (call $call (i32.const 512) (i32.const 600) (i32.const 7)))))
`
	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(self, mustWat(watSrc), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, rentFund) // fund the pre-reentry kv write's deposit

		preKeys := len(acc.Context.Keys)
		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err == nil {
			t.Fatal("self-reentry was allowed")
		}
		reloaded, _ := getContract(txn, self)
		if got := len(reloaded.Context.Keys); got != preKeys {
			t.Fatalf("journal write survived the reentry abort: %d -> %d keys", preKeys, got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestDepChainDepth locks the dependency-chain bound: a chain of
// maxDepChainDepth static deps links fine, one more is rejected — the
// growth path a malicious dependency graph would use to stall linking.
func TestDepChainDepth(t *testing.T) {
	db := newTestDB(t)

	// build chain: c[n-1] imports c[n], ..., c0 imports c1; main imports c0
	build := func(txn *bbolt.Tx, n int) ngtypes.Address {
		addrs := make([]ngtypes.Address, n)
		for i := range addrs {
			addrs[i] = testAddr(byte(0x90 + i))
		}
		// deepest first: c[n-1] has no deps
		for i := n - 1; i >= 0; i-- {
			imports := ""
			if i < n-1 {
				imports = `(import "` + addrs[i+1].String() + `" "leaf" (func $next))`
			}
			watSrc := `(module ` + imports + ` (func (export "leaf")))`
			acc := ngtypes.NewContract(addrs[i], mustWat(watSrc), nil)
			acc.SetActive(true)
			putContract(t, txn, acc, 0)
		}
		mainAddr := testAddr(0x8f)
		mainWat := `(module
			(import "` + addrs[0].String() + `" "leaf" (func $c0))
			(func (export "ng:main") (call $c0)))`
		acc := ngtypes.NewContract(mainAddr, mustWat(mainWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)
		return mainAddr
	}

	err := db.Update(func(txn *bbolt.Tx) error {
		// exactly at the bound: links and runs
		mainAddr := build(txn, maxDepChainDepth)
		acc, _ := getContract(txn, mainAddr)
		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			t.Fatalf("chain of %d deps rejected: %v", maxDepChainDepth, err)
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("chain of %d deps failed to run: %v", maxDepChainDepth, err)
		}

		// one beyond: rejected at link time
		mainAddr = build(txn, maxDepChainDepth+1)
		acc, _ = getContract(txn, mainAddr)
		if _, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1); err == nil {
			t.Fatalf("chain of %d deps was allowed", maxDepChainDepth+1)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestStorageDepositGolden locks the exact storage bond a known write pays.
// Like gas, the deposit is consensus state (active from genesis): kvWat stores
// key "key" + value "val" = 6 bytes, so it must lock 6 * DepositPerByte from
// the contract into the escrow — and supply must be conserved.
func TestStorageDepositGolden(t *testing.T) {
	db := newTestDB(t)
	addr := testAddr(0x78)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, mustWat(kvWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, rentFund)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		want := new(big.Int).Mul(ngtypes.DepositPerByte, big.NewInt(6)) // 6 stored bytes
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(want) != 0 {
			t.Fatalf("escrow deposit = %s, golden %s (DepositPerByte*6)", got, want)
		}
		if got := getBalance(txn, addr); got.Cmp(new(big.Int).Sub(big.NewInt(rentFund), want)) != 0 {
			t.Fatalf("contract balance after deposit = %s, want %d-%s", got, rentFund, want)
		}
		// supply conservation: nothing minted or burned, only moved to escrow
		total := new(big.Int).Add(getBalance(txn, addr), getBalance(txn, ngtypes.StorageDepositEscrow))
		if total.Cmp(big.NewInt(rentFund)) != 0 {
			t.Fatalf("supply not conserved: contract+escrow = %s, want %d", total, rentFund)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestModuleCacheDistinctContracts: distinct sources get distinct cached
// templates (no cross-contamination), while each source stays memoized to one.
func TestModuleCacheDistinctContracts(t *testing.T) {
	a, err := templateFor(mustWat(kvWat))
	if err != nil {
		t.Fatal(err)
	}
	b, err := templateFor(mustWat(logWat))
	if err != nil {
		t.Fatal(err)
	}
	c, err := templateFor(mustWat(`(module (func (export "ng:main")))`))
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a == c || b == c {
		t.Fatal("distinct sources collapsed onto a shared template")
	}
	if again, _ := templateFor(mustWat(kvWat)); again != a {
		t.Fatal("a source was not memoized to one template")
	}
}

// TestServiceCallSuccess complements TestServiceTrapRollback: when the callee
// RETURNS, the caller's journaled writes (and their storage deposit) COMMIT.
func TestServiceCallSuccess(t *testing.T) {
	db := newTestDB(t)

	privCallee, _ := ngtypes.GenerateKey()
	privCaller, _ := ngtypes.GenerateKey()
	calleeAddr := ngtypes.NewAddress(privCallee)
	callerAddr := ngtypes.NewAddress(privCaller)

	calleeWat := `(module (func (export "noop")))`
	callerWat := `(module
		(import "` + calleeAddr.String() + `" "noop" (func $noop))
		(import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
		(memory 1)
		(data (i32.const 0) "okok")
		(func (export "ng:main")
			(drop (call $set (i32.const 0) (i32.const 2) (i32.const 2) (i32.const 2))) ;; "ok"->"ok"
			(call $noop)))`

	err := db.Update(func(txn *bbolt.Tx) error {
		callee := ngtypes.NewContract(calleeAddr, mustWat(calleeWat), nil)
		callee.SetActive(true)
		putContract(t, txn, callee, 0) // callee writes nothing, needs no deposit
		caller := ngtypes.NewContract(callerAddr, mustWat(callerWat), nil)
		caller.SetActive(true)
		putContract(t, txn, caller, rentFund)

		callerAcc, _ := getContract(txn, callerAddr)
		vm, err := NewVM(txn, callerAcc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("a successful service call must not abort: %v", err)
		}

		reloaded, _ := getContract(txn, callerAddr)
		if got := string(reloaded.Context.Get("ok")); got != "ok" {
			t.Fatalf("caller write did not commit after a successful service call: %q", got)
		}
		// its 4-byte deposit ("ok"+"ok") landed in the escrow
		want := new(big.Int).Mul(ngtypes.DepositPerByte, big.NewInt(4))
		if got := getBalance(txn, ngtypes.StorageDepositEscrow); got.Cmp(want) != 0 {
			t.Fatalf("committed deposit = %s, want %s", got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestReservedKeyTrap: a contract writing a system-reserved ("_"-prefixed) kv
// key must trap — the reserved namespace (`_active`/`_deps`/`_refs`/`_rent`)
// is protocol-owned and a contract must never forge it.
func TestReservedKeyTrap(t *testing.T) {
	db := newTestDB(t)
	addr := testAddr(0x79)

	// key "_active" (7 bytes) at offset 0, a 1-byte value
	watSrc := `(module
		(import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
		(memory 1)
		(data (i32.const 0) "_activeX")
		(func (export "ng:main")
			(drop (call $set (i32.const 0) (i32.const 7) (i32.const 7) (i32.const 1)))))`

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, mustWat(watSrc), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, rentFund)

		vm, err := NewVM(txn, acc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err == nil {
			t.Fatal("a write to a reserved key must trap")
		}
		// the account must still be active — the reserved flag was not clobbered
		reloaded, _ := getContract(txn, addr)
		if !reloaded.IsActive() {
			t.Fatal("reserved-key write corrupted the _active flag")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
