package ngstate

import (
	"encoding/binary"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// A full DeFi composition showcase across FOUR independent deployers:
//
//	A publishes usdt     — an allowance-based token (erc20-alike)
//	B publishes dex      — swaps usdt for native NG at a fixed rate,
//	                       depending on A's token
//	C publishes lending  — a usdt lending pool, depending on A's token
//	D publishes leverage — the strategy: borrow usdt from C, approve
//	                       B, swap into native NG (depends on A, B, C)
//
// Every party IS its address: contracts import each other directly by
// <deployer bs58 address>, ledger keys are 32-byte addresses, and
// addresses cross service boundaries through the buf slots
// (slot 1 = primary address argument, slot 2 = secondary)

// usdtTokenWat: balances keyed by the 32-byte address; allowances by
// the 64-byte owner||spender key.
// Memory: 0..64 key area, 64..72 value scratch, 96..128 / 128..160 addr scratch
const usdtTokenWat = `
(module
  (import "address" "get_caller" (func $caller (param i32) (result i32)))
  (import "env" "buf_get" (func $bget (param i32 i32) (result i32)))
  (import "kv" "get" (func $kvget (param i32 i32 i32) (result i32)))
  (import "kv" "set" (func $kvset (param i32 i32 i32 i32) (result i32)))
  (memory 1)

  (func $load (param $klen i32) (result i64)
    (i64.store (i32.const 64) (i64.const 0))
    (drop (call $kvget (i32.const 0) (local.get $klen) (i32.const 64)))
    (i64.load (i32.const 64)))
  (func $store (param $klen i32) (param $v i64)
    (i64.store (i32.const 64) (local.get $v))
    (drop (call $kvset (i32.const 0) (local.get $klen) (i32.const 64) (i32.const 8))))
  (func $cp32 (param $dst i32) (param $src i32)
    (i64.store (local.get $dst) (i64.load (local.get $src)))
    (i64.store (i32.add (local.get $dst) (i32.const 8)) (i64.load (i32.add (local.get $src) (i32.const 8))))
    (i64.store (i32.add (local.get $dst) (i32.const 16)) (i64.load (i32.add (local.get $src) (i32.const 16))))
    (i64.store (i32.add (local.get $dst) (i32.const 24)) (i64.load (i32.add (local.get $src) (i32.const 24)))))

  ;; moves $amt from the address at 96 to the address at 128
  (func $xfer (param $amt i64) (result i32)
    (call $cp32 (i32.const 0) (i32.const 96))
    (if (i64.lt_u (call $load (i32.const 32)) (local.get $amt))
      (then (return (i32.const 0))))
    (call $store (i32.const 32) (i64.sub (call $load (i32.const 32)) (local.get $amt)))
    (call $cp32 (i32.const 0) (i32.const 128))
    (call $store (i32.const 32) (i64.add (call $load (i32.const 32)) (local.get $amt)))
    (i32.const 1))

  ;; mint to the address in slot 1
  (func (export "mint_to") (param $amt i64)
    (drop (call $bget (i32.const 1) (i32.const 0)))
    (call $store (i32.const 32) (i64.add (call $load (i32.const 32)) (local.get $amt))))

  ;; caller pays the address in slot 1
  (func (export "transfer") (param $amt i64) (result i32)
    (drop (call $caller (i32.const 96)))
    (drop (call $bget (i32.const 1) (i32.const 128)))
    (call $xfer (local.get $amt)))

  ;; caller allows the spender in slot 1
  (func (export "approve") (param $amt i64)
    (drop (call $caller (i32.const 0)))
    (drop (call $bget (i32.const 1) (i32.const 32)))
    (call $store (i32.const 64) (local.get $amt)))

  ;; caller (the approved spender) moves slot1 -> slot2
  (func (export "transfer_from") (param $amt i64) (result i32)
    (drop (call $bget (i32.const 1) (i32.const 96)))
    (drop (call $bget (i32.const 2) (i32.const 128)))
    (call $cp32 (i32.const 0) (i32.const 96))
    (drop (call $caller (i32.const 32)))
    (if (i64.lt_u (call $load (i32.const 64)) (local.get $amt))
      (then (return (i32.const 0))))
    (call $store (i32.const 64) (i64.sub (call $load (i32.const 64)) (local.get $amt)))
    (call $xfer (local.get $amt)))

  ;; balance of the address in slot 1
  (func (export "balance_of") (result i64)
    (drop (call $bget (i32.const 1) (i32.const 0)))
    (call $load (i32.const 32))))
`

// dexWatFor sells native NG for usdt at the fixed rate 2 usdt = 1 NG:
// it pulls the buyer's approved usdt into its own pool and pays out
// from its own coin balance
func dexWatFor(token ngtypes.Address) string {
	return `
(module
  (import "address" "get_caller" (func $caller (param i32) (result i32)))
  (import "address" "get_host" (func $host (param i32) (result i32)))
  (import "env" "buf_set" (func $bset (param i32 i32 i32) (result i32)))
  (import "coin" "transfer" (func $pay (param i32 i64) (result i32)))
  (import "` + token.String() + `" "transfer_from"
    (func $pull (param i64) (result i32)))
  (memory 1)

  ;; 0..32 caller, 32..64 host
  (func (export "buy_coin") (param $usdt i64) (result i32)
    (drop (call $caller (i32.const 0)))
    (drop (call $host (i32.const 32)))
    (drop (call $bset (i32.const 1) (i32.const 0) (i32.const 32)))
    (drop (call $bset (i32.const 2) (i32.const 32) (i32.const 32)))
    (if (i32.eqz (call $pull (local.get $usdt)))
      (then (return (i32.const 0))))
    (call $pay (i32.const 0) (i64.div_u (local.get $usdt) (i64.const 2)))))
`
}

// lendingWatFor is a usdt pool: deposits pull approved tokens in,
// borrows record the debt and pay out of the pool
func lendingWatFor(token ngtypes.Address) string {
	return `
(module
  (import "address" "get_caller" (func $caller (param i32) (result i32)))
  (import "address" "get_host" (func $host (param i32) (result i32)))
  (import "env" "buf_set" (func $bset (param i32 i32 i32) (result i32)))
  (import "kv" "get" (func $kvget (param i32 i32 i32) (result i32)))
  (import "kv" "set" (func $kvset (param i32 i32 i32 i32) (result i32)))
  (import "` + token.String() + `" "transfer"
    (func $send (param i64) (result i32)))
  (import "` + token.String() + `" "transfer_from"
    (func $pull (param i64) (result i32)))
  (memory 1)

  ;; loan key: borrower address at 0..32; val 64..72; scratch 96/128
  (func $load (result i64)
    (i64.store (i32.const 64) (i64.const 0))
    (drop (call $kvget (i32.const 0) (i32.const 32) (i32.const 64)))
    (i64.load (i32.const 64)))
  (func $store (param $v i64)
    (i64.store (i32.const 64) (local.get $v))
    (drop (call $kvset (i32.const 0) (i32.const 32) (i32.const 64) (i32.const 8))))

  (func (export "deposit") (param $amt i64) (result i32)
    (drop (call $caller (i32.const 96)))
    (drop (call $host (i32.const 128)))
    (drop (call $bset (i32.const 1) (i32.const 96) (i32.const 32)))
    (drop (call $bset (i32.const 2) (i32.const 128) (i32.const 32)))
    (call $pull (local.get $amt)))

  (func (export "borrow") (param $amt i64) (result i32)
    (drop (call $caller (i32.const 0)))
    (call $store (i64.add (call $load) (local.get $amt)))
    (drop (call $bset (i32.const 1) (i32.const 0) (i32.const 32)))
    (call $send (local.get $amt)))

  (func (export "loan_of") (result i64)
    (drop (call $bset (i32.const 3) (i32.const 0) (i32.const 0)))
    (call $load)))
`
}

// strategyWatFor opens the position: borrow 100 usdt from the lending
// pool, approve the dex as spender, swap into 50 native NG
func strategyWatFor(token, dex, lending ngtypes.Address) string {
	return `
(module
  (import "env" "buf_set" (func $bset (param i32 i32 i32) (result i32)))
  (import "` + lending.String() + `" "borrow"
    (func $borrow (param i64) (result i32)))
  (import "` + token.String() + `" "approve"
    (func $approve (param i64)))
  (import "` + dex.String() + `" "buy_coin"
    (func $buy (param i64) (result i32)))
  (memory 1)
  (data (i32.const 0) "` + watBytes(dex[:]) + `")

  (func (export "main")
    (drop (call $borrow (i64.const 100)))
    (drop (call $bset (i32.const 1) (i32.const 0) (i32.const 32)))
    (call $approve (i64.const 100))
    (drop (call $buy (i64.const 100)))))
`
}

func TestLeverageShowcase(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	// four INDEPENDENT deployers
	privA, _ := ngtypes.GenerateKey() // token project
	privB, _ := ngtypes.GenerateKey() // dex project
	privC, _ := ngtypes.GenerateKey() // lending project
	privD, _ := ngtypes.GenerateKey() // leverage strategist
	addrA, addrB := ngtypes.NewAddress(privA), ngtypes.NewAddress(privB)
	addrC, addrD := ngtypes.NewAddress(privC), ngtypes.NewAddress(privD)

	leU64 := func(raw []byte) uint64 {
		if len(raw) != 8 {
			return 0
		}
		return binary.LittleEndian.Uint64(raw)
	}

	err := db.Update(func(txn *bbolt.Tx) error {
		// the token launches with the lending pool pre-seeded: 1000 usdt
		seeded := ngtypes.NewContractContext()
		seeded.Set(string(addrC[:]), func() []byte {
			raw := make([]byte, 8)
			binary.LittleEndian.PutUint64(raw, 1000)
			return raw
		}())

		usdt := ngtypes.NewContract(addrA, mustWat(usdtTokenWat), seeded)
		putContract(t, txn, usdt, 100)
		dex := ngtypes.NewContract(addrB, mustWat(dexWatFor(addrA)), nil)
		putContract(t, txn, dex, 1000) // the dex pool holds native NG
		lending := ngtypes.NewContract(addrC, mustWat(lendingWatFor(addrA)), nil)
		putContract(t, txn, lending, 100)
		leverage := ngtypes.NewContract(addrD, mustWat(strategyWatFor(addrA, addrB, addrC)), nil)
		putContract(t, txn, leverage, 100)

		lock := func(priv *ngtypes.PrivateKey, who string) {
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.ActivateTx, 1,
				ngtypes.Address{}, nil, big.NewInt(1), nil, nil)
			if err := tx.Signature(priv); err != nil {
				t.Fatal(err)
			}
			if err := state.handleActivate(txn, tx, 1, nil); err != nil {
				t.Fatalf("lock %s: %v", who, err)
			}
		}

		// activation order follows the dependency DAG
		lock(privA, "usdt")
		lock(privB, "dex")
		lock(privC, "lending")
		lock(privD, "leverage")

		// the reference ledger across projects:
		// usdt <- dex, lending, leverage; dex <- leverage; lending <- leverage
		for addr, want := range map[ngtypes.Address]uint64{addrA: 3, addrB: 1, addrC: 1} {
			acc, _ := getContract(txn, addr)
			if got := getRefCount(acc); got != want {
				t.Fatalf("refcount(%s) = %d, want %d", addr, got, want)
			}
		}

		// open the leveraged position
		leverageAcc, _ := getContract(txn, addrD)
		vm, err := NewVM(txn, leverageAcc, fakeTransactTx(ngtypes.Address{}, nil), 1)
		if err != nil {
			t.Fatalf("NewVM: %v", err)
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("strategy run: %v", err)
		}

		// the usdt ledger (inside the TOKEN's kv):
		// lending 1000-100=900, dex +100, leverage borrowed then spent = 0
		usdtAcc, _ := getContract(txn, addrA)
		if got := leU64(usdtAcc.Context.Get(string(addrC[:]))); got != 900 {
			t.Fatalf("usdt[lending] = %d, want 900", got)
		}
		if got := leU64(usdtAcc.Context.Get(string(addrB[:]))); got != 100 {
			t.Fatalf("usdt[dex] = %d, want 100", got)
		}
		if got := leU64(usdtAcc.Context.Get(string(addrD[:]))); got != 0 {
			t.Fatalf("usdt[leverage] = %d, want 0", got)
		}

		// the debt book (inside the LENDING's kv)
		lendingAcc, _ := getContract(txn, addrC)
		if got := leU64(lendingAcc.Context.Get(string(addrD[:]))); got != 100 {
			t.Fatalf("loan[leverage] = %d, want 100", got)
		}

		// native NG moved from the dex pool to the strategist
		// (each deployer paid a 1-coin lock fee on activation)
		if got := getBalance(txn, addrB); got.Int64() != 1000-1-50 {
			t.Fatalf("dex NG = %d, want 949", got.Int64())
		}
		if got := getBalance(txn, addrD); got.Int64() != 100-1+50 {
			t.Fatalf("leverage NG = %d, want 149", got.Int64())
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
