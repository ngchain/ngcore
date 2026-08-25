package ngstate

import (
	"bytes"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// TestTraceInternalCalls pins the internal-call trace: contract A makes a
// native transfer AND a dynamic contract.call to B, and both surface as
// ordered trace frames with the right kind, emitter, target and depth
func TestTraceInternalCalls(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	callee := testAddr(0xbe)
	caller := testAddr(0xca)

	// B: on ping it makes its OWN native transfer (a nested internal tx,
	// depth 1) to the zero address (@800), value 1 (@700)
	calleeWat := `
(module
  (import "coin" "transfer" (func $transfer (param i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 700) "\01")
  (func (export "ping")
    (drop (call $transfer (i32.const 800) (i32.const 700)))))
`
	// A: transfer 5 to the zero address (@800), then call B.ping (@512 target)
	callerWat := `
(module
  (import "contract" "call" (func $call (param i32 i32 i32) (result i32)))
  (import "coin" "transfer" (func $transfer (param i32 i32) (result i32)))
  (import "tx" "get_extra" (func $args (param i32) (result i32)))
  (memory 1)
  (data (i32.const 600) "\c6\84\70\69\6e\67\80")
  (data (i32.const 700) "\05")
  (func (export "ng:main")
    (drop (call $args (i32.const 512)))
    (drop (call $transfer (i32.const 800) (i32.const 700)))
    (drop (call $call (i32.const 512) (i32.const 600) (i32.const 7)))))
`

	err := db.Update(func(txn *bbolt.Tx) error {
		b := ngtypes.NewContract(callee, mustWat(calleeWat), nil)
		b.SetActive(true)
		putContract(t, txn, b, 0)
		a := ngtypes.NewContract(caller, mustWat(callerWat), nil)
		a.SetActive(true)
		putContract(t, txn, a, 0)

		// fund A and B so their transfers succeed
		if err := setBalance(txn, nil, caller, big.NewInt(1000)); err != nil {
			return err
		}
		if err := setBalance(txn, nil, callee, big.NewInt(1000)); err != nil {
			return err
		}

		args := append([]byte{}, callee[:]...)
		tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1, caller, nil, nil, args, nil)

		vm, err := NewVM(txn, a, tx, 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			return err
		}

		trace := vm.Trace()
		if len(trace) != 3 {
			t.Fatalf("trace has %d frames, want 3: %+v", len(trace), trace)
		}

		// frame 0: A's native transfer to the zero address, value 5, depth 0
		tr := trace[0]
		if tr.Type != "transfer" || tr.Depth != 0 || !tr.Ok {
			t.Fatalf("frame0 = %+v, want transfer depth 0 ok", tr)
		}
		if !bytes.Equal(tr.From, caller[:]) || !bytes.Equal(tr.To, make([]byte, 32)) {
			t.Fatalf("frame0 from/to = %x/%x", tr.From, tr.To)
		}
		if new(big.Int).SetBytes(reverse(tr.Value)).Int64() != 5 {
			t.Fatalf("frame0 value = %x, want 5", tr.Value)
		}

		// frame 1: the dynamic call A -> B (ping), depth 0
		cl := trace[1]
		if cl.Type != "call" || cl.Depth != 0 || !cl.Ok || cl.Method != "ping" {
			t.Fatalf("frame1 = %+v, want call depth 0 ok ping", cl)
		}
		if !bytes.Equal(cl.From, caller[:]) || !bytes.Equal(cl.To, callee[:]) {
			t.Fatalf("frame1 from/to = %x/%x, want %x/%x", cl.From, cl.To, caller[:], callee[:])
		}

		// frame 2: B's OWN transfer, nested one level deeper (depth 1)
		nested := trace[2]
		if nested.Type != "transfer" || nested.Depth != 1 || !nested.Ok {
			t.Fatalf("frame2 = %+v, want transfer depth 1 ok", nested)
		}
		if !bytes.Equal(nested.From, callee[:]) {
			t.Fatalf("frame2 from = %x, want B %x", nested.From, callee[:])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = state
}

// TestTraceReentryBlocked pins that a re-entrancy-blocked internal call is
// still visible in the trace (as a failed frame), not silently dropped
func TestTraceReentryBlocked(t *testing.T) {
	db := newTestDB(t)
	self := testAddr(0xaa)

	// main loads its OWN address and dynamically calls itself -> the
	// re-entrancy guard blocks it and the run aborts
	selfWat := `
(module
  (import "contract" "call" (func $call (param i32 i32 i32) (result i32)))
  (import "address" "get_host" (func $host (param i32) (result i32)))
  (memory 1)
  (data (i32.const 600) "\c6\84\70\69\6e\67\80")
  (func (export "ng:main")
    (drop (call $host (i32.const 512)))
    (drop (call $call (i32.const 512) (i32.const 600) (i32.const 7)))))
`
	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(self, mustWat(selfWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1, self, nil, nil, self[:], nil)
		vm, err := NewVM(txn, acc, tx, 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err == nil {
			t.Fatal("a reentrant self-call must fail the run")
		}

		trace := vm.Trace()
		if len(trace) != 1 {
			t.Fatalf("trace = %d frames, want 1 (the blocked call): %+v", len(trace), trace)
		}
		if trace[0].Type != "call" || trace[0].Ok || !bytes.Equal(trace[0].To, self[:]) {
			t.Fatalf("frame = %+v, want a blocked call to self", trace[0])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// reverse flips byte order (LE money -> BE for big.Int)
func reverse(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return out
}
