package ngstate

import (
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/ngchain/secp256k1"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// A full DeFi composition showcase across FOUR independent deployers:
//
//	A publishes `usdt`     — an allowance-based token (erc20-alike)
//	B publishes `dex`      — swaps usdt for native NG at a fixed rate,
//	                         depending on A's token
//	C publishes `lending`  — a usdt lending pool, depending on A's token
//	D publishes `leverage` — the strategy: borrow usdt from C, approve
//	                         B, swap into native NG (depends on A, B, C)
//
// account nums: usdt 700, dex 710, lending 720, leverage 730

// usdtWat: balances keyed by the 8-byte LE account num; allowances by
// the 16-byte owner||spender key. Scratch memory: 0..16 key, 16..24 val
const usdtWat = `
(module
  (import "account" "get_caller" (func $caller (result i64)))
  (import "kv" "get" (func $kvget (param i32 i32 i32) (result i32)))
  (import "kv" "set" (func $kvset (param i32 i32 i32 i32) (result i32)))
  (memory 1)

  (func $load (param $klen i32) (result i64)
    (i64.store (i32.const 16) (i64.const 0))
    (drop (call $kvget (i32.const 0) (local.get $klen) (i32.const 16)))
    (i64.load (i32.const 16)))
  (func $store (param $klen i32) (param $v i64)
    (i64.store (i32.const 16) (local.get $v))
    (drop (call $kvset (i32.const 0) (local.get $klen) (i32.const 16) (i32.const 8))))
  (func $balkey (param $who i64)
    (i64.store (i32.const 0) (local.get $who)))
  (func $allowkey (param $owner i64) (param $spender i64)
    (i64.store (i32.const 0) (local.get $owner))
    (i64.store (i32.const 8) (local.get $spender)))

  (func $xfer (param $from i64) (param $to i64) (param $amt i64) (result i32)
    (call $balkey (local.get $from))
    (if (i64.lt_u (call $load (i32.const 8)) (local.get $amt))
      (then (return (i32.const 0))))
    (call $balkey (local.get $from))
    (call $store (i32.const 8) (i64.sub (call $load (i32.const 8)) (local.get $amt)))
    (call $balkey (local.get $to))
    (call $store (i32.const 8) (i64.add (call $load (i32.const 8)) (local.get $amt)))
    (i32.const 1))

  (func (export "mint_to") (param $to i64) (param $amt i64)
    (call $balkey (local.get $to))
    (call $store (i32.const 8) (i64.add (call $load (i32.const 8)) (local.get $amt))))

  (func (export "transfer") (param $to i64) (param $amt i64) (result i32)
    (call $xfer (call $caller) (local.get $to) (local.get $amt)))

  (func (export "approve") (param $spender i64) (param $amt i64)
    (call $allowkey (call $caller) (local.get $spender))
    (call $store (i32.const 16) (local.get $amt)))

  (func (export "transfer_from") (param $from i64) (param $to i64) (param $amt i64) (result i32)
    (call $allowkey (local.get $from) (call $caller))
    (if (i64.lt_u (call $load (i32.const 16)) (local.get $amt))
      (then (return (i32.const 0))))
    (call $allowkey (local.get $from) (call $caller))
    (call $store (i32.const 16) (i64.sub (call $load (i32.const 16)) (local.get $amt)))
    (call $xfer (local.get $from) (local.get $to) (local.get $amt)))

  (func (export "balance_of") (param $who i64) (result i64)
    (call $balkey (local.get $who))
    (call $load (i32.const 8))))
`

// dexWatFor sells native NG for usdt at the fixed rate 2 usdt = 1 NG:
// it pulls the buyer's approved usdt into its own pool and pays out
// from its own coin balance
func dexWatFor(tokenDeployer ngtypes.Address) string {
	return `
(module
  (import "account" "get_caller" (func $caller (result i64)))
  (import "account" "get_host" (func $host (result i64)))
  (import "coin" "transfer" (func $pay (param i64 i64) (result i32)))
  (import "service/` + tokenDeployer.String() + `.usdt" "transfer_from"
    (func $pull (param i64 i64 i64) (result i32)))

  (func (export "buy_coin") (param $usdt i64) (result i32)
    (if (i32.eqz (call $pull (call $caller) (call $host) (local.get $usdt)))
      (then (return (i32.const 0))))
    (call $pay (call $caller) (i64.div_u (local.get $usdt) (i64.const 2)))))
`
}

// lendingWatFor is a usdt pool: deposits pull approved tokens in,
// borrows record the debt and pay out of the pool
func lendingWatFor(tokenDeployer ngtypes.Address) string {
	return `
(module
  (import "account" "get_caller" (func $caller (result i64)))
  (import "account" "get_host" (func $host (result i64)))
  (import "kv" "get" (func $kvget (param i32 i32 i32) (result i32)))
  (import "kv" "set" (func $kvset (param i32 i32 i32 i32) (result i32)))
  (import "service/` + tokenDeployer.String() + `.usdt" "transfer"
    (func $send (param i64 i64) (result i32)))
  (import "service/` + tokenDeployer.String() + `.usdt" "transfer_from"
    (func $pull (param i64 i64 i64) (result i32)))
  (memory 1)

  (func $loankey (param $who i64)
    (i64.store (i32.const 0) (local.get $who)))
  (func $load (result i64)
    (i64.store (i32.const 16) (i64.const 0))
    (drop (call $kvget (i32.const 0) (i32.const 8) (i32.const 16)))
    (i64.load (i32.const 16)))
  (func $store (param $v i64)
    (i64.store (i32.const 16) (local.get $v))
    (drop (call $kvset (i32.const 0) (i32.const 8) (i32.const 16) (i32.const 8))))

  (func (export "deposit") (param $amt i64) (result i32)
    (call $pull (call $caller) (call $host) (local.get $amt)))

  (func (export "borrow") (param $amt i64) (result i32)
    (call $loankey (call $caller))
    (call $store (i64.add (call $load) (local.get $amt)))
    (call $send (call $caller) (local.get $amt)))

  (func (export "loan_of") (param $who i64) (result i64)
    (call $loankey (local.get $who))
    (call $load)))
`
}

// leverageWatFor opens the position: borrow 100 usdt from the lending
// pool, approve the dex, swap into 50 native NG
func leverageWatFor(tokenDeployer, dexDeployer, lendingDeployer ngtypes.Address) string {
	return `
(module
  (import "service/` + lendingDeployer.String() + `.lending" "borrow"
    (func $borrow (param i64) (result i32)))
  (import "service/` + tokenDeployer.String() + `.usdt" "approve"
    (func $approve (param i64 i64)))
  (import "service/` + dexDeployer.String() + `.dex" "buy_coin"
    (func $buy (param i64) (result i32)))

  (func (export "main")
    (drop (call $borrow (i64.const 100)))
    (call $approve (i64.const 710) (i64.const 100)) ;; spender: the dex account
    (drop (call $buy (i64.const 100)))))
`
}

func TestLeverageShowcase(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	// four INDEPENDENT deployers
	privA, _ := secp256k1.GeneratePrivateKey() // token project
	privB, _ := secp256k1.GeneratePrivateKey() // dex project
	privC, _ := secp256k1.GeneratePrivateKey() // lending project
	privD, _ := secp256k1.GeneratePrivateKey() // leverage strategist
	addrA, addrB := ngtypes.NewAddress(privA), ngtypes.NewAddress(privB)
	addrC, addrD := ngtypes.NewAddress(privC), ngtypes.NewAddress(privD)

	numKey := func(num uint64) string {
		raw := make([]byte, 8)
		binary.LittleEndian.PutUint64(raw, num)
		return string(raw)
	}
	leU64 := func(raw []byte) uint64 {
		if len(raw) != 8 {
			return 0
		}
		return binary.LittleEndian.Uint64(raw)
	}

	err := db.Update(func(txn *bbolt.Tx) error {
		// the token launches with the lending pool pre-seeded: 1000 usdt
		seeded := ngtypes.NewAccountContext()
		seeded.Set(numKey(720), func() []byte {
			raw := make([]byte, 8)
			binary.LittleEndian.PutUint64(raw, 1000)
			return raw
		}())

		usdt := ngtypes.NewAccount(700, addrA, []byte(usdtWat), seeded)
		putAccount(t, txn, usdt, 100)
		dex := ngtypes.NewAccount(710, addrB, []byte(dexWatFor(addrA)), nil)
		putAccount(t, txn, dex, 1000) // the dex pool holds native NG
		lending := ngtypes.NewAccount(720, addrC, []byte(lendingWatFor(addrA)), nil)
		putAccount(t, txn, lending, 100)
		leverage := ngtypes.NewAccount(730, addrD, []byte(leverageWatFor(addrA, addrB, addrC)), nil)
		putAccount(t, txn, leverage, 100)

		lock := func(convener uint64, priv *secp256k1.PrivateKey, name string) {
			tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.LockTx, 1, ngtypes.AccountNum(convener),
				nil, nil, big.NewInt(1), []byte(name), nil)
			if err := tx.Signature(priv); err != nil {
				t.Fatal(err)
			}
			if err := state.handleLock(txn, tx, 1); err != nil {
				t.Fatalf("lock %d (%s): %v", convener, name, err)
			}
		}

		// activation order follows the dependency DAG
		lock(700, privA, "usdt")
		lock(710, privB, "dex")
		lock(720, privC, "lending")
		lock(730, privD, "leverage")

		// the reference ledger across projects:
		// usdt <- dex, lending, leverage; dex <- leverage; lending <- leverage
		for num, want := range map[uint64]uint64{700: 3, 710: 1, 720: 1} {
			acc, _ := getAccountByNum(txn, ngtypes.AccountNum(num))
			if got := getRefCount(acc); got != want {
				t.Fatalf("refcount(%d) = %d, want %d", num, got, want)
			}
		}

		// open the leveraged position
		leverageAcc, _ := getAccountByNum(txn, 730)
		vm, err := NewVM(txn, leverageAcc, fakeTransactTx(nil, nil), 1)
		if err != nil {
			t.Fatalf("NewVM: %v", err)
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("strategy run: %v", err)
		}

		// the usdt ledger (inside the TOKEN's kv):
		// lending 1000-100=900, dex +100, leverage borrowed then spent = 0
		usdtAcc, _ := getAccountByNum(txn, 700)
		if got := leU64(usdtAcc.Context.Get(numKey(720))); got != 900 {
			t.Fatalf("usdt[lending] = %d, want 900", got)
		}
		if got := leU64(usdtAcc.Context.Get(numKey(710))); got != 100 {
			t.Fatalf("usdt[dex] = %d, want 100", got)
		}
		if got := leU64(usdtAcc.Context.Get(numKey(730))); got != 0 {
			t.Fatalf("usdt[leverage] = %d, want 0", got)
		}

		// the debt book (inside the LENDING's kv)
		lendingAcc, _ := getAccountByNum(txn, 720)
		if got := leU64(lendingAcc.Context.Get(numKey(730))); got != 100 {
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
