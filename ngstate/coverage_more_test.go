package ngstate

import (
	"errors"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngblocks"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// unsignedTx builds an UNSIGNED tx of the given type, so the per-type
// self checks (CheckDeploy/...) fail before any state is
// consulted
func unsignedTx(txType ngtypes.TxType, to ngtypes.Address, value, fee *big.Int, extra []byte) *ngtypes.FullTx {
	return ngtypes.NewTx(ngtypes.ZERONET, txType, 1, to, value, fee, extra, nil)
}

// TestCheckPerTypeSelfCheckFails calls the per-type check helpers DIRECTLY
// with an unsigned tx: each one must fail at its own CheckX self check,
// the branch CheckTx never reaches because it rejects unsigned txs first
func TestCheckPerTypeSelfCheckFails(t *testing.T) {
	db := newTestDB(t)

	err := db.View(func(txn *bbolt.Tx) error {
		// an empty-code deploy (a destroy) is still a deploy: its self check
		// rejects the unsigned tx before any state is consulted
		if err := checkDeploy(txn, unsignedTx(ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			destroyExtra())); err == nil {
			t.Fatal("checkDeploy must reject an unsigned empty-code (destroy) tx")
		}
		if err := checkDeploy(txn, unsignedTx(ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(logWat)))); err == nil {
			t.Fatal("checkDeploy must reject an unsigned tx")
		}
		if err := checkTransaction(txn, unsignedTx(ngtypes.TransactTx, testAddr(0x01), big.NewInt(1), big.NewInt(1), nil)); err == nil {
			t.Fatal("checkTransaction must reject an unsigned tx")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCheckPerTypeNoSlot: a signed, funded empty-code deploy (a destroy)
// against an address with NO live contract slot fails as nothing to
// destroy (a branch the "unsigned" refusals above never reach)
func TestCheckPerTypeNoSlot(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(100)); err != nil {
			return err
		}

		destroy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1), destroyExtra())
		if err := checkDeploy(txn, destroy); !errors.Is(err, ErrNothingToDestroy) {
			t.Fatalf("destroy on a slotless address: got %v, want ErrNothingToDestroy", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCheckPerTypeUnfunded: a signed but UNFUNDED deploy fails at
// fromWithBalance (the expense exceeds the zero balance)
func TestCheckPerTypeUnfunded(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()

	err := db.View(func(txn *bbolt.Tx) error {
		deploy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(logWat)))
		if err := checkDeploy(txn, deploy); !errors.Is(err, ErrTxrBalanceInsufficient) {
			t.Fatalf("unfunded deploy: got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCheckBlockTxsBadNonGenerate: a non-generate tx that fails CheckTx
// propagates through CheckBlockTxs (the CheckTx call site there)
func TestCheckBlockTxsBadNonGenerate(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()

	err := db.Update(func(txn *bbolt.Tx) error {
		// a valid miner generate (so the block passes the generate-set gate),
		// then a signed but unfunded transact that fails inside CheckTx. The
		// reveal's commitment is seeded so the tx reaches the balance check
		minerAddr := ngtypes.NewAddress(priv)
		gen := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
			minerAddr, ngtypes.GetBlockReward(1), big.NewInt(0), nil, nil)
		if err := gen.Signature(priv); err != nil {
			return err
		}
		broke := signedTx(t, priv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(1), big.NewInt(1), nil)
		seedCommit(t, txn, priv, broke)
		genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
		header := *genesis.BlockHeader
		header.Height = 1
		header.Coinbase = minerAddr[:]
		block := &ngtypes.FullBlock{BlockHeader: &header, Txs: []*ngtypes.FullTx{gen, broke}}
		if err := CheckBlockTxs(txn, block); !errors.Is(err, ErrTxrBalanceInsufficient) {
			t.Fatalf("CheckBlockTxs with an unfunded transact: got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestHandlePerTypeSelfCheckFails drives the handle* helpers directly with
// unsigned txs: the CheckX self-check branch inside each handler
func TestHandlePerTypeSelfCheckFails(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := state.handleDeploy(txn, unsignedTx(ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(logWat))), 1, nil); err == nil {
			t.Fatal("handleDeploy must reject an unsigned tx")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestHandleTxsDispatch drives EACH tx-type branch of HandleTxs so the
// per-type dispatch arms (destroy, deploy) are exercised through the
// top-level entry point
func TestHandleTxsDispatch(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		gen := signedTx(t, priv, ngtypes.GenerateTx, addr, big.NewInt(1000), big.NewInt(0), nil)
		// the deployed module exports `upgrade`, so the empty-code deploy
		// (the destroy) is authorized to clear its own slot
		deploy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(upgradeableWat)))
		destroy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1), destroyExtra())
		seedCommit(t, txn, priv, deploy)
		seedCommit(t, txn, priv, destroy)

		// one HandleTxs call routing through every non-transact arm
		if err := state.HandleTxs(txn, 1, gen, deploy, destroy); err != nil {
			t.Fatalf("HandleTxs dispatch: %v", err)
		}
		if _, err := getContract(txn, addr); err == nil {
			t.Fatal("the slot must be gone after the destroy arm")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestMatureBalanceFromSnapshot puts a snapshot at the mature height so
// GetMatureBalanceByAddress resolves the address's balance through the
// snapshot loop instead of the genesis-fallback zero
func TestMatureBalanceFromSnapshot(t *testing.T) {
	db := newTestDB(t)
	ngblocks.Init(db, ngtypes.ZERONET)
	state := newTestState(t, db)

	addr := testAddr(0x71)

	// pin a tip well past the maturity window so GetMatureHeight lands on a
	// non-zero snapshot height (height 0 is always the genesis fallback)
	tip := uint64(ngtypes.MatureHeight) * 3
	err := state.Update(func(txn *bbolt.Tx) error {
		return txn.Bucket(storage.BlockBucketName).Put(storage.LatestHeightTag, utils.PackUint64LE(tip))
	})
	if err != nil {
		t.Fatal(err)
	}

	matureHeight := ngtypes.GetMatureHeight(tip)
	if matureHeight == 0 {
		t.Fatal("expected a non-zero mature height for the test setup")
	}
	sheet := ngtypes.NewSheet(ngtypes.ZERONET, matureHeight, []byte("mature-hash"),
		[]*ngtypes.Balance{{Address: addr, Amount: big.NewInt(4242)}},
		[]*ngtypes.Contract{}, []*ngtypes.RegisteredKey{})
	state.SnapshotManager.PutSnapshot(matureHeight, sheet.BlockHash, sheet)

	mature, err := state.GetMatureBalanceByAddress(addr)
	if err != nil {
		t.Fatalf("GetMatureBalanceByAddress: %v", err)
	}
	if mature.Int64() != 4242 {
		t.Fatalf("mature balance = %s, want 4242", mature)
	}

	// an address absent from the mature snapshot resolves to zero
	other, err := state.GetMatureBalanceByAddress(testAddr(0x72))
	if err != nil {
		t.Fatalf("GetMatureBalanceByAddress(other): %v", err)
	}
	if other.Sign() != 0 {
		t.Fatalf("absent address mature balance = %s, want 0", other)
	}
}

// TestStateHelperEdges covers the low-level slot/code helpers' edge
// branches: a corrupt stored slot, short code-bucket entries and an
// already-registered pubkey
func TestStateHelperEdges(t *testing.T) {
	db := newTestDB(t)

	addr := testAddr(0x81)

	err := db.Update(func(txn *bbolt.Tx) error {
		// a corrupt contract slot: getContract must surface the decode error
		if err := txn.Bucket(storage.ContractBucketName).Put(addr[:], []byte{0xff, 0xff}); err != nil {
			return err
		}
		if _, err := getContract(txn, addr); err == nil {
			t.Fatal("a corrupt slot must fail to decode")
		}

		// a short (<8 byte) code-bucket entry: loadCode and releaseCode
		// both treat it as absent without panicking
		shortHash := make([]byte, 32)
		if err := txn.Bucket(storage.CodeBucketName).Put(shortHash, []byte{0x01}); err != nil {
			return err
		}
		if got := loadCode(txn, shortHash); got != nil {
			t.Fatal("loadCode on a short entry must return nil")
		}
		releaseCode(txn, shortHash, false) // must be a no-op, not a panic

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// hostHappyWat exercises the host host-function HAPPY paths the
// guard-rail probe never reaches: a present-key kv read, a real kv delete,
// a valid log.error message, and the tx views (get_from/get_to/get_code)
// under a SIGNED calling tx
const hostHappyWat = `
(module
  (import "log" "error" (func $err (param i32 i32)))
  (import "kv" "set" (func $set (param i32 i32 i32 i32) (result i32)))
  (import "kv" "get_size" (func $getsize (param i32 i32) (result i32)))
  (import "kv" "del" (func $del (param i32 i32) (result i32)))
  (import "tx" "get_from" (func $tfrom (param i32) (result i32)))
  (import "tx" "get_to" (func $tto (param i32) (result i32)))
  (import "address" "get_host" (func $host (param i32) (result i32)))
  (import "contract" "get_code" (func $code (param i32 i32) (result i32)))
  (memory 1)
  ;; 0..2 "kk"  2..5 "msg"
  (data (i32.const 0) "kkmsg")
  (func (export "main")
    ;; seed key "kk" then read its size back (present-key read)
    (drop (call $set (i32.const 0) (i32.const 2) (i32.const 0) (i32.const 2)))
    (drop (call $getsize (i32.const 0) (i32.const 2)))
    ;; a real delete of an existing key
    (drop (call $del (i32.const 0) (i32.const 2)))
    ;; a valid (in-bounds) error message
    (call $err (i32.const 2) (i32.const 3))
    ;; tx views: from (signed tx), to
    (drop (call $tfrom (i32.const 64)))
    (drop (call $tto (i32.const 128)))
    ;; get_code of self: write self address at 256, then read own code
    (drop (call $host (i32.const 256)))
    (drop (call $code (i32.const 256) (i32.const 1024)))))
`

// TestHostHappyPaths runs the happy-path probe under a signed tx so
// tx.get_from succeeds and the present-key / real-delete / valid-log
// branches all execute
func TestHostHappyPaths(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, mustWat(hostHappyWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		// a SIGNED transact tx whose To is the contract itself
		tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
			addr, big.NewInt(0), big.NewInt(0), nil, nil)
		if err := tx.Signature(priv); err != nil {
			return err
		}

		vm, err := NewVM(txn, acc, tx, 1)
		if err != nil {
			return err
		}
		if err := vm.Run(VMEntryOnTx); err != nil {
			t.Fatalf("host happy-path probe trapped: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRegisterPubKeyAlreadyRegistered: registering the same full-envelope
// key twice takes the "already registered" early return the second time
func TestRegisterPubKeyAlreadyRegistered(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateSchemeKey(ngtypes.SchemeMLDSA44)
	// a full-envelope (non-recovery) tx carries the public key, so it
	// registers on chain
	tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
		testAddr(0x01), big.NewInt(1), big.NewInt(0), nil, nil)
	if err := tx.Signature(priv); err != nil {
		t.Fatal(err)
	}

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := registerPubKey(txn, nil, tx); err != nil {
			return err
		}
		// the registry now holds the key under its envelope-derived address
		if txn.Bucket(storage.KeyRegistryBucketName).Get(ngtypes.NewAddress(priv).Bytes()) == nil {
			t.Fatal("the key must be registered after the first envelope")
		}
		// the second registration hits the already-registered short-circuit
		if err := registerPubKey(txn, nil, tx); err != nil {
			t.Fatalf("re-registering must be a no-op, got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
