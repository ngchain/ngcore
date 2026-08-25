package ngstate

import (
	"errors"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
)

// TestHandleTxsFullLifecycle drives a whole contract lifecycle through
// the block-level HandleTxs dispatcher: mine, deploy (goes live at once),
// transact, destroy
func TestHandleTxsFullLifecycle(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		gen := signedTx(t, priv, ngtypes.GenerateTx, addr, big.NewInt(1000), big.NewInt(0), nil)
		// the module exports `upgrade`, so its own empty-code deploy (the
		// destroy) is authorized to clear the slot at the end
		deploy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(upgradeableWat)))
		pay := signedTx(t, priv, ngtypes.TransactTx, addr, big.NewInt(5), big.NewInt(1), nil)
		destroy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1), destroyExtra())
		seedCommit(t, txn, priv, deploy)
		seedCommit(t, txn, priv, pay)
		seedCommit(t, txn, priv, destroy)

		if err := state.HandleTxs(txn, 1, gen, deploy, pay, destroy); err != nil {
			t.Fatalf("HandleTxs lifecycle: %v", err)
		}

		// 1000 mined, fees burned on 3 txs, the 5-value round-tripped
		if got := getBalance(txn, addr); got.Int64() != 997 {
			t.Fatalf("final balance = %s, want 997", got)
		}
		// the slot was destroyed at the end
		if _, err := getContract(txn, addr); err == nil {
			t.Fatal("the slot must be destroyed")
		}
		// the transact ran the contract while it was active
		runs, err := GetTxRuns(txn, pay.GetHash())
		if err != nil {
			return err
		}
		if len(runs) != 1 || !runs[0].Ok {
			t.Fatalf("the transact must have run the contract: %+v", runs)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestHandleTxsRefusals covers the dispatcher's error paths
func TestHandleTxsRefusals(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, _ := ngtypes.GenerateKey()

	err := db.Update(func(txn *bbolt.Tx) error {
		invalid := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.InvalidTx, 1, ngtypes.Address{}, nil, big.NewInt(0), nil, nil)
		if err := state.HandleTxs(txn, 1, invalid); !errors.Is(err, ngtypes.ErrTxTypeInvalid) {
			t.Fatalf("InvalidTx: got %v", err)
		}

		odd := signedTx(t, priv, ngtypes.TxType(200), ngtypes.Address{}, nil, big.NewInt(0), nil)
		if err := state.HandleTxs(txn, 1, odd); !errors.Is(err, ngtypes.ErrTxTypeInvalid) {
			t.Fatalf("unknown type: got %v", err)
		}

		// an unsigned generate is a trusted uncle-reward system mint at the
		// state layer (block-level CheckBlockTxs is what binds it to a real
		// uncle); HandleTxs credits it without a signature
		unsignedGen := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
			testAddr(0x09), big.NewInt(10), big.NewInt(0), nil, nil)
		if err := state.HandleTxs(txn, 1, unsignedGen); err != nil {
			t.Fatalf("unsigned uncle-reward generate should mint: %v", err)
		}
		if getBalance(txn, testAddr(0x09)).Cmp(big.NewInt(10)) != 0 {
			t.Fatal("unsigned generate did not credit its recipient")
		}

		// spending from an empty address fails at the charge (its reveal
		// commitment is seeded so it reaches the charge, not the reveal gate)
		broke := signedTx(t, priv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(10), big.NewInt(1), nil)
		seedCommit(t, txn, priv, broke)
		if err := state.HandleTxs(txn, 1, broke); !errors.Is(err, ErrTxrBalanceInsufficient) {
			t.Fatalf("unfunded transact: got %v", err)
		}

		// an empty-code deploy (a destroy) with no live slot fails
		destroy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(0), destroyExtra())
		if err := state.handleDeploy(txn, destroy, 1, nil); !errors.Is(err, ErrNothingToDestroy) {
			t.Fatalf("destroy without a slot: got %v, want ErrNothingToDestroy", err)
		}

		// a deploy whose extra is not a deploy payload fails
		badDeploy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(0), []byte{0xff})
		if err := state.handleDeploy(txn, badDeploy, 1, nil); err == nil {
			t.Fatal("a broken deploy extra must fail")
		}

		// an unsigned deploy fails its self check
		unsignedDeploy := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.DeployTx, 1,
			ngtypes.Address{}, nil, big.NewInt(0), nil, nil)
		if err := state.handleDeploy(txn, unsignedDeploy, 1, nil); err == nil {
			t.Fatal("an unsigned deploy must fail")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestHandleDestroyRefd: a referenced slot survives a destroy attempt
func TestHandleDestroyRefd(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := activeContract(addr, mustWat(logWat))
		setRefCount(acc, 1)
		putContract(t, txn, acc, 100)

		destroy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1), destroyExtra())
		if err := state.handleDeploy(txn, destroy, 1, nil); !errors.Is(err, ErrContractRefdBy) {
			t.Fatalf("destroy refd slot: got %v", err)
		}
		if _, err := getContract(txn, addr); err != nil {
			t.Fatal("the refd slot must survive")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestHandleDeploySelfDep: deploying a module which imports its own
// address is refused
func TestHandleDeploySelfDep(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(100)); err != nil {
			return err
		}

		selfWat := `(module (import "` + addr.String() + `" "f" (func $f)) (func (export "main")))`
		deploy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(selfWat)))
		if err := state.handleDeploy(txn, deploy, 1, nil); !errors.Is(err, ErrDepSelf) {
			t.Fatalf("self dep deploy: got %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRecordRunBrokenReceipt: a corrupted stored receipt makes the
// append fail, which must only log — the run itself succeeds
func TestRecordRunBrokenReceipt(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	addr := testAddr(0x51)

	err := db.Update(func(txn *bbolt.Tx) error {
		acc := ngtypes.NewContract(addr, mustWat(kvWat), nil)
		acc.SetActive(true)
		putContract(t, txn, acc, 0)

		tx := fakeTransactTx(addr, nil)
		if err := txn.Bucket(storage.ReceiptBucketName).Put(tx.GetHash(), []byte{0xff}); err != nil {
			return err
		}

		// must not panic nor fail the tx despite the receipt error
		state.runContract(txn, addr, tx, VMEntryOnTx, 1, nil)

		// the contract still ran and flushed
		reloaded, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		if got := string(reloaded.Context.Get("key")); got != "val" {
			t.Fatalf("run lost despite the receipt failure: %q", got)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRunContractSkips: runContract silently skips addresses without a
// runnable contract
func TestRunContractSkips(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	err := db.Update(func(txn *bbolt.Tx) error {
		// no slot at all
		tx := fakeTransactTx(testAddr(0x61), nil)
		state.runContract(txn, testAddr(0x61), tx, VMEntryOnTx, 1, nil)
		if runs, _ := GetTxRuns(txn, tx.GetHash()); runs != nil {
			t.Fatal("a slotless address must not record runs")
		}

		// an inactive slot
		putContract(t, txn, ngtypes.NewContract(testAddr(0x62), mustWat(logWat), nil), 0)
		state.runContract(txn, testAddr(0x62), tx, VMEntryOnTx, 1, nil)
		if runs, _ := GetTxRuns(txn, tx.GetHash()); runs != nil {
			t.Fatal("an inactive slot must not run")
		}

		// an active slot with a broken source: the vm build fails, the
		// failure lands in the receipt
		bad := ngtypes.NewContract(testAddr(0x63), []byte{0x00, 0x61, 0x73, 0x6d, 0xff}, nil)
		bad.SetActive(true)
		putContract(t, txn, bad, 0)
		badTx := fakeTransactTx(testAddr(0x63), nil)
		state.runContract(txn, testAddr(0x63), badTx, VMEntryOnTx, 1, nil)
		runs, err := GetTxRuns(txn, badTx.GetHash())
		if err != nil {
			return err
		}
		if len(runs) != 1 || runs[0].Ok || runs[0].Error == "" {
			t.Fatalf("broken source run = %+v", runs)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
