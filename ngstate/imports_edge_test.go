package ngstate

import (
	"strings"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// hostEdgeWat probes every host module's guard rails from inside the
// sandbox: out-of-bounds pointers, reserved kv keys (reads), bad buf
// slots, unknown addresses — every probe must return 0 without
// trapping, while the happy-path sibling calls stay intact.
//
// $z sums returns that MUST be zero; $bad counts must-be-nonzero calls
// that came back zero. Both land in kv for the host-side assertion.
// 70000 is past the 1-page (65536) linear memory.
const hostEdgeWat = `
(module
  (import "log" "debug" (func $dbg (param i32 i32)))
  (import "log" "error" (func $err (param i32 i32)))
  (import "log" "emit" (func $emit (param i32 i32 i32 i32) (result i32)))
  (import "env" "buf_set" (func $bset (param i32 i32 i32) (result i32)))
  (import "env" "buf_size" (func $bsize (param i32) (result i32)))
  (import "env" "buf_get" (func $bget (param i32 i32) (result i32)))
  (import "env" "get_gas" (func $gas (result i64)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (import "kv" "get" (func $get (param i32 i32 i32) (result i32)))
  (import "kv" "get_size" (func $getsize (param i32 i32) (result i32)))
  (import "kv" "count" (func $count (param i32 i32) (result i32)))
  (import "kv" "key_size_at" (func $ksizeat (param i32 i32 i32) (result i32)))
  (import "kv" "key_at" (func $keyat (param i32 i32 i32 i32) (result i32)))
  (import "kv" "del" (func $del (param i32 i32) (result i32)))
  (import "coin" "get_balance" (func $bal (param i32 i32) (result i32)))
  (import "coin" "transfer" (func $xfer (param i32 i32) (result i32)))
  (import "crypto" "keccak256" (func $keccak (param i32 i32 i32) (result i32)))
  (import "crypto" "verify" (func $verify (param i32 i32 i32 i32 i32 i32) (result i32)))
  (import "crypto" "addr_of" (func $addrof (param i32 i32 i32 i32) (result i32)))
  (import "address" "get_size" (func $asize (result i32)))
  (import "address" "get_host" (func $host (param i32) (result i32)))
  (import "address" "get_caller" (func $caller (param i32) (result i32)))
  (import "contract" "call" (func $call (param i32 i32 i32) (result i32)))
  (import "contract" "is_active" (func $active (param i32) (result i32)))
  (import "contract" "get_code_size" (func $codesize (param i32) (result i32)))
  (import "contract" "get_code" (func $code (param i32 i32) (result i32)))
  (import "contract" "code_hash" (func $codehash (param i32 i32) (result i32)))
  (import "tx" "get_hash_size" (func $thashsize (result i32)))
  (import "tx" "get_hash" (func $thash (param i32) (result i32)))
  (import "tx" "get_network" (func $tnet (result i32)))
  (import "tx" "get_height" (func $theight (result i64)))
  (import "tx" "get_paid" (func $tpaid (param i32) (result i32)))
  (import "tx" "get_from" (func $tfrom (param i32) (result i32)))
  (import "tx" "get_to" (func $tto (param i32) (result i32)))
  (import "tx" "get_fee_size" (func $tfeesize (result i32)))
  (import "tx" "get_fee" (func $tfee (param i32) (result i32)))
  (import "tx" "get_extra_size" (func $textrasize (result i32)))
  (import "tx" "get_extra" (func $textra (param i32) (result i32)))
  (memory 1)
  ;; 0..2 "zz", 2..4 "_r", 4..7 "bad", 7..10 "sum"
  (data (i32.const 0) "zz_rbadsum")
  ;; 512..544 stays all-zero: the zero address
  (func (export "main")
    (local $z i32) (local $bad i32)
    ;; --- log: oob pointers must be swallowed
    (call $dbg (i32.const 70000) (i32.const 4))
    (call $err (i32.const 70000) (i32.const 4))
    ;; --- seed one kv entry: key "zz" = 1 byte
    (i32.store8 (i32.const 100) (i32.const 7))
    (if (i32.eqz (call $set (i32.const 0) (i32.const 2) (i32.const 100) (i32.const 1)))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    ;; --- emit guards
    (local.set $z (i32.add (local.get $z)
      (call $emit (i32.const 70000) (i32.const 4) (i32.const 0) (i32.const 0))))
    (local.set $z (i32.add (local.get $z)
      (call $emit (i32.const 0) (i32.const 65) (i32.const 0) (i32.const 0))))
    (local.set $z (i32.add (local.get $z)
      (call $emit (i32.const 0) (i32.const 2) (i32.const 70000) (i32.const 4))))
    ;; --- env buf slots
    (local.set $z (i32.add (local.get $z) (call $bset (i32.const 8) (i32.const 0) (i32.const 2))))
    (local.set $z (i32.add (local.get $z) (call $bset (i32.const 0) (i32.const 0) (i32.const 4097))))
    (local.set $z (i32.add (local.get $z) (call $bset (i32.const 0) (i32.const 70000) (i32.const 2))))
    (local.set $z (i32.add (local.get $z) (call $bsize (i32.const 8))))
    (local.set $z (i32.add (local.get $z) (call $bget (i32.const 8) (i32.const 16))))
    (if (i32.eqz (call $bset (i32.const 0) (i32.const 0) (i32.const 2)))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (if (i32.ne (call $bsize (i32.const 0)) (i32.const 2))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (if (i32.ne (call $bget (i32.const 0) (i32.const 16)) (i32.const 2))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (local.set $z (i32.add (local.get $z) (call $bget (i32.const 0) (i32.const 70000))))
    (drop (call $gas))
    ;; --- kv guards: reserved keys read as absent, oob pointers fail
    (local.set $z (i32.add (local.get $z) (call $getsize (i32.const 2) (i32.const 2))))
    (local.set $z (i32.add (local.get $z) (call $get (i32.const 2) (i32.const 2) (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $getsize (i32.const 70000) (i32.const 2))))
    (local.set $z (i32.add (local.get $z) (call $get (i32.const 70000) (i32.const 2) (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $get (i32.const 0) (i32.const 2) (i32.const 70000))))
    (local.set $z (i32.add (local.get $z) (call $count (i32.const 70000) (i32.const 2))))
    (local.set $z (i32.add (local.get $z) (call $ksizeat (i32.const 70000) (i32.const 2) (i32.const 0))))
    (local.set $z (i32.add (local.get $z) (call $ksizeat (i32.const 0) (i32.const 2) (i32.const 99))))
    (local.set $z (i32.add (local.get $z) (call $keyat (i32.const 70000) (i32.const 2) (i32.const 0) (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $keyat (i32.const 0) (i32.const 2) (i32.const 99) (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $keyat (i32.const 0) (i32.const 2) (i32.const 0) (i32.const 70000))))
    (if (i32.ne (call $ksizeat (i32.const 0) (i32.const 2) (i32.const 0)) (i32.const 2))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (if (i32.ne (call $keyat (i32.const 0) (i32.const 2) (i32.const 0) (i32.const 16)) (i32.const 2))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (local.set $z (i32.add (local.get $z) (call $del (i32.const 70000) (i32.const 2))))
    ;; --- coin
    (local.set $z (i32.add (local.get $z) (call $bal (i32.const 70000) (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $bal (i32.const 512) (i32.const 70000))))
    (if (i32.ne (call $bal (i32.const 512) (i32.const 16)) (i32.const 32))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (local.set $z (i32.add (local.get $z) (call $xfer (i32.const 70000) (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $xfer (i32.const 512) (i32.const 70000))))
    ;; broke: this contract owns 0 coins, sending 1 must fail
    (i32.store8 (i32.const 200) (i32.const 1))
    (local.set $z (i32.add (local.get $z) (call $xfer (i32.const 512) (i32.const 200))))
    ;; --- crypto
    (local.set $z (i32.add (local.get $z) (call $keccak (i32.const 70000) (i32.const 4) (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $keccak (i32.const 0) (i32.const 4) (i32.const 70000))))
    (if (i32.ne (call $keccak (i32.const 0) (i32.const 4) (i32.const 16)) (i32.const 32))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (local.set $z (i32.add (local.get $z)
      (call $verify (i32.const 1) (i32.const 70000) (i32.const 33) (i32.const 16) (i32.const 16) (i32.const 65))))
    (local.set $z (i32.add (local.get $z)
      (call $verify (i32.const 1) (i32.const 16) (i32.const 33) (i32.const 70000) (i32.const 16) (i32.const 65))))
    (local.set $z (i32.add (local.get $z)
      (call $verify (i32.const 1) (i32.const 16) (i32.const 33) (i32.const 16) (i32.const 70000) (i32.const 65))))
    (local.set $z (i32.add (local.get $z)
      (call $verify (i32.const 1) (i32.const 16) (i32.const 33) (i32.const 16) (i32.const 16) (i32.const 65))))
    (local.set $z (i32.add (local.get $z) (call $addrof (i32.const 1) (i32.const 70000) (i32.const 33) (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $addrof (i32.const 1) (i32.const 16) (i32.const 33) (i32.const 70000))))
    (if (i32.ne (call $addrof (i32.const 1) (i32.const 16) (i32.const 33) (i32.const 16)) (i32.const 32))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    ;; --- address
    (if (i32.ne (call $asize) (i32.const 32))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (local.set $z (i32.add (local.get $z) (call $host (i32.const 70000))))
    (local.set $z (i32.add (local.get $z) (call $caller (i32.const 70000))))
    (if (i32.ne (call $host (i32.const 128)) (i32.const 32))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (if (i32.ne (call $caller (i32.const 300)) (i32.const 32))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    ;; --- contract introspection: self at 128, the zero address at 512
    (local.set $z (i32.add (local.get $z) (call $active (i32.const 70000))))
    (local.set $z (i32.add (local.get $z) (call $active (i32.const 512))))
    (if (i32.eqz (call $active (i32.const 128)))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (local.set $z (i32.add (local.get $z) (call $codesize (i32.const 70000))))
    (local.set $z (i32.add (local.get $z) (call $codesize (i32.const 512))))
    (if (i32.eqz (call $codesize (i32.const 128)))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (local.set $z (i32.add (local.get $z) (call $code (i32.const 70000) (i32.const 1024))))
    (local.set $z (i32.add (local.get $z) (call $code (i32.const 512) (i32.const 1024))))
    (local.set $z (i32.add (local.get $z) (call $code (i32.const 128) (i32.const 70000))))
    (local.set $z (i32.add (local.get $z) (call $codehash (i32.const 70000) (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $codehash (i32.const 512) (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $codehash (i32.const 128) (i32.const 70000))))
    (if (i32.ne (call $codehash (i32.const 128) (i32.const 16)) (i32.const 32))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    ;; --- dynamic call guards
    (local.set $z (i32.add (local.get $z) (call $call (i32.const 70000) (i32.const 0) (i32.const 0))))
    (local.set $z (i32.add (local.get $z) (call $call (i32.const 512) (i32.const 0) (i32.const 0))))
    (local.set $z (i32.add (local.get $z) (call $call (i32.const 128) (i32.const 70000) (i32.const 4))))
    ;; --- tx views
    (if (i32.ne (call $thashsize) (i32.const 32))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (local.set $z (i32.add (local.get $z) (call $thash (i32.const 70000))))
    (if (i32.ne (call $thash (i32.const 16)) (i32.const 32))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (drop (call $tnet))
    (drop (i32.wrap_i64 (call $theight)))
    (local.set $z (i32.add (local.get $z) (call $tpaid (i32.const 70000))))
    ;; the calling tx is unsigned: get_from must fail with 0
    (local.set $z (i32.add (local.get $z) (call $tfrom (i32.const 16))))
    (local.set $z (i32.add (local.get $z) (call $tto (i32.const 70000))))
    (if (i32.ne (call $tto (i32.const 16)) (i32.const 32))
      (then (local.set $bad (i32.add (local.get $bad) (i32.const 1)))))
    (drop (call $tfeesize))
    (local.set $z (i32.add (local.get $z) (call $tfee (i32.const 70000))))
    (drop (call $textrasize))
    (local.set $z (i32.add (local.get $z) (call $textra (i32.const 70000))))
    ;; --- verdicts
    (i32.store8 (i32.const 400) (local.get $bad))
    (drop (call $set (i32.const 4) (i32.const 3) (i32.const 400) (i32.const 1)))
    (i32.store8 (i32.const 401) (local.get $z))
    (drop (call $set (i32.const 7) (i32.const 3) (i32.const 401) (i32.const 1)))))
`

// TestHostGuardRails runs the sandbox probe contract: every
// out-of-bounds or malformed host call must fail soft (return 0) and
// every well-formed sibling must still work
func TestHostGuardRails(t *testing.T) {
	db := newTestDB(t)

	addr := testAddr(0xe1)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, mustWat(hostEdgeWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		vm, err := NewVM(txn, acc, fakeTransactTx(addr, nil), 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("the probe contract trapped: %v", err)
		}

		reloaded, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if got := reloaded.Context.Get("bad"); len(got) != 1 || got[0] != 0 {
			t.Fatalf("%d happy-path host calls failed", got[0])
		}
		if got := reloaded.Context.Get("sum"); len(got) != 1 || got[0] != 0 {
			t.Fatalf("must-be-zero host calls summed to %d", got[0])
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestKVReservedKeyTraps: writing (or deleting) a "_"-prefixed reserved
// key traps the call and rolls the journal back
func TestKVReservedKeyTraps(t *testing.T) {
	db := newTestDB(t)

	// each contract first performs a legit write, then hits the reserved key
	setWat := `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "ok_r")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 2) (i32.const 0) (i32.const 1)))
    (drop (call $set (i32.const 2) (i32.const 2) (i32.const 0) (i32.const 1)))))
`
	delWat := `
(module
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (import "kv" "del" (func $del (param i32 i32) (result i32)))
  (memory 1)
  (data (i32.const 0) "ok_r")
  (func (export "main")
    (drop (call $set (i32.const 0) (i32.const 2) (i32.const 0) (i32.const 1)))
    (drop (call $del (i32.const 2) (i32.const 2)))))
`

	for name, watSrc := range map[string]string{"set": setWat, "del": delWat} {
		addr := testAddr(0xe2)
		err := db.Update(func(txn *bbolt.Tx) error {
			acc := ngtypes.NewContract(addr, mustWat(watSrc), nil)
			acc.SetActive(true)
			putContract(t, txn, acc, 0)

			vm, err := NewVM(txn, acc, fakeTransactTx(addr, nil), 1)
			if err != nil {
				return err
			}
			err = vm.Run(VMEntryOnTx)
			if err == nil {
				t.Fatalf("kv.%s on a reserved key must trap", name)
			}
			if !strings.Contains(err.Error(), "reserved key") {
				t.Fatalf("kv.%s trap reason lost: %v", name, err)
			}

			// the legit write from before the trap must be rolled back
			reloaded, err := getContract(txn, addr)
			if err != nil {
				return err
			}
			if got := reloaded.Context.Get("ok"); len(got) != 0 {
				t.Fatalf("kv.%s: journal leaked through the trap: %v", name, got)
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
