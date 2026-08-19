package ngstate

import (
	"errors"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// signedTx builds and signs a tx of the given type from priv
func signedTx(t *testing.T, priv *ngtypes.PrivateKey, txType ngtypes.TxType, to ngtypes.Address,
	value, fee *big.Int, extra []byte,
) *ngtypes.FullTx {
	t.Helper()

	tx := ngtypes.NewTx(ngtypes.ZERONET, txType, 1, to, value, fee, extra, nil)
	if err := tx.Signature(priv); err != nil {
		t.Fatal(err)
	}
	return tx
}

// TestCheckBlockTxs covers the block-level tx validation: the generate
// branch, the per-tx checks and the refusal paths
func TestCheckBlockTxs(t *testing.T) {
	db := newTestDB(t)

	minerPriv, _ := ngtypes.GenerateKey()
	minerAddr := ngtypes.NewAddress(minerPriv)
	userPriv, _ := ngtypes.GenerateKey()
	userAddr := ngtypes.NewAddress(userPriv)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, userAddr, big.NewInt(100)); err != nil {
			return err
		}

		// a minimal block shell: CheckBlockTxs only consults the height
		// and the txs (never mutate the shared genesis block singleton)
		blockAt := func(height uint64, txs ...*ngtypes.FullTx) *ngtypes.FullBlock {
			genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
			header := *genesis.BlockHeader
			header.Height = height
			return &ngtypes.FullBlock{BlockHeader: &header, Txs: txs}
		}

		// a valid generate plus a valid transact
		gen := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
			minerAddr, ngtypes.GetBlockReward(1), big.NewInt(0), nil, nil)
		if err := gen.Signature(minerPriv); err != nil {
			return err
		}
		pay := signedTx(t, userPriv, ngtypes.TransactTx, minerAddr, big.NewInt(1), big.NewInt(1), nil)

		if err := CheckBlockTxs(txn, blockAt(1, gen, pay)); err != nil {
			t.Fatalf("valid block refused: %v", err)
		}

		// an unsigned tx is refused before anything else
		unsigned := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
			minerAddr, big.NewInt(1), big.NewInt(0), nil, nil)
		if err := CheckBlockTxs(txn, blockAt(1, unsigned)); !errors.Is(err, ngtypes.ErrTxUnsigned) {
			t.Fatalf("unsigned tx: got %v, want ErrTxUnsigned", err)
		}

		// an oversized extra is refused
		fat := signedTx(t, userPriv, ngtypes.TransactTx, minerAddr,
			big.NewInt(1), big.NewInt(0), make([]byte, ngtypes.TxMaxExtraSize+1))
		if err := CheckBlockTxs(txn, blockAt(1, fat)); !errors.Is(err, ngtypes.ErrTxExtraExcess) {
			t.Fatalf("fat tx: got %v, want ErrTxExtraExcess", err)
		}

		// a generate with the wrong reward is refused
		greedy := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
			minerAddr, new(big.Int).Add(ngtypes.GetBlockReward(1), big.NewInt(1)), big.NewInt(0), nil, nil)
		if err := greedy.Signature(minerPriv); err != nil {
			return err
		}
		if err := CheckBlockTxs(txn, blockAt(1, greedy)); err == nil {
			t.Fatal("an over-rewarding generate must be refused")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCheckTxPerType covers CheckTx's dispatch and every per-type check's
// refusal paths
func TestCheckTxPerType(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)
	poorPriv, _ := ngtypes.GenerateKey()

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, addr, big.NewInt(100)); err != nil {
			return err
		}

		// unsigned refused
		unsigned := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
			addr, big.NewInt(1), big.NewInt(0), nil, nil)
		if err := CheckTx(txn, unsigned); !errors.Is(err, ngtypes.ErrTxSignInvalid) {
			t.Fatalf("unsigned: got %v", err)
		}

		// oversized extra refused
		fat := signedTx(t, priv, ngtypes.TransactTx, addr,
			big.NewInt(1), big.NewInt(0), make([]byte, ngtypes.TxMaxExtraSize+1))
		if err := CheckTx(txn, fat); !errors.Is(err, ngtypes.ErrTxExtraExcess) {
			t.Fatalf("fat: got %v", err)
		}

		// an unknown tx type is refused
		odd := signedTx(t, priv, ngtypes.TxType(200), ngtypes.Address{}, nil, big.NewInt(0), nil)
		if err := CheckTx(txn, odd); !errors.Is(err, ngtypes.ErrTxTypeInvalid) {
			t.Fatalf("odd type: got %v", err)
		}

		// transact: fine when funded, refused when the balance misses
		pay := signedTx(t, priv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(1), big.NewInt(1), nil)
		if err := CheckTx(txn, pay); err != nil {
			t.Fatalf("funded transact: %v", err)
		}
		broke := signedTx(t, poorPriv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(1), big.NewInt(1), nil)
		if err := CheckTx(txn, broke); !errors.Is(err, ErrTxrBalanceInsufficient) {
			t.Fatalf("unfunded transact: got %v", err)
		}

		// destroy: no slot -> refused
		destroy := signedTx(t, priv, ngtypes.DestroyTx, ngtypes.Address{}, nil, big.NewInt(1), nil)
		if err := CheckTx(txn, destroy); err == nil {
			t.Fatal("destroy without a slot must be refused")
		}

		// open an inactive slot: destroy and commit both check out
		if err := setContract(txn, ngtypes.NewContract(addr, mustWat(logWat), nil)); err != nil {
			return err
		}
		if err := CheckTx(txn, destroy); err != nil {
			t.Fatalf("destroy on an inactive slot: %v", err)
		}
		commit := signedTx(t, priv, ngtypes.CommitTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(logWat)))
		if err := CheckTx(txn, commit); err != nil {
			t.Fatalf("commit: %v", err)
		}
		// a commit whose extra is not a commit payload is refused
		badCommit := signedTx(t, priv, ngtypes.CommitTx, ngtypes.Address{}, nil, big.NewInt(1), []byte{0xff})
		if err := CheckTx(txn, badCommit); err == nil {
			t.Fatal("a broken commit extra must be refused")
		}

		// deactivate on an inactive slot is refused
		deactivate := signedTx(t, priv, ngtypes.DeactivateTx, ngtypes.Address{}, nil, big.NewInt(1), nil)
		if err := CheckTx(txn, deactivate); !errors.Is(err, ErrContractNotActive) {
			t.Fatalf("deactivate inactive: got %v", err)
		}

		// activate checks out on the inactive slot
		activate := signedTx(t, priv, ngtypes.ActivateTx, ngtypes.Address{}, nil, big.NewInt(1), nil)
		if err := CheckTx(txn, activate); err != nil {
			t.Fatalf("activate: %v", err)
		}

		// flip the slot active: activate/destroy refused, deactivate fine
		slot, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		slot.SetActive(true)
		if err := setContract(txn, slot); err != nil {
			return err
		}
		if err := CheckTx(txn, activate); !errors.Is(err, ErrContractActive) {
			t.Fatalf("activate active: got %v", err)
		}
		if err := CheckTx(txn, destroy); !errors.Is(err, ErrContractActive) {
			t.Fatalf("destroy active: got %v", err)
		}
		if err := CheckTx(txn, deactivate); err != nil {
			t.Fatalf("deactivate active: %v", err)
		}

		// a referenced slot can be neither deactivated nor destroyed
		setRefCount(slot, 2)
		if err := setContract(txn, slot); err != nil {
			return err
		}
		if err := CheckTx(txn, deactivate); !errors.Is(err, ErrContractRefdBy) {
			t.Fatalf("deactivate refd: got %v", err)
		}
		slot.SetActive(false)
		if err := setContract(txn, slot); err != nil {
			return err
		}
		if err := CheckTx(txn, destroy); !errors.Is(err, ErrContractRefdBy) {
			t.Fatalf("destroy refd: got %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCheckTxGeneratePanics: CheckTx must never be handed a generate tx
func TestCheckTxGeneratePanics(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	gen := signedTx(t, priv, ngtypes.GenerateTx, ngtypes.NewAddress(priv),
		ngtypes.GetBlockReward(1), big.NewInt(0), nil)

	err := db.View(func(txn *bbolt.Tx) error {
		defer func() {
			if recover() == nil {
				t.Error("CheckTx(generate) must panic")
			}
		}()
		_ = CheckTx(txn, gen)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCheckActivateDeps covers the dependency validation at lock time:
// self-deps, unknown deps and inactive deps are all refused
func TestCheckActivateDeps(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)
	depPriv, _ := ngtypes.GenerateKey()
	depAddr := ngtypes.NewAddress(depPriv)

	activate := func(t *testing.T) *ngtypes.FullTx {
		return signedTx(t, priv, ngtypes.ActivateTx, ngtypes.Address{}, nil, big.NewInt(1), nil)
	}

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, addr, big.NewInt(100)); err != nil {
			return err
		}

		// a contract importing its OWN address
		selfWat := `(module (import "` + addr.String() + `" "f" (func $f)) (func (export "main")))`
		if err := setContract(txn, ngtypes.NewContract(addr, mustWat(selfWat), nil)); err != nil {
			return err
		}
		if err := checkActivate(txn, activate(t)); !errors.Is(err, ErrDepSelf) {
			t.Fatalf("self dep: got %v", err)
		}

		// an unknown dependency address
		depWat := `(module (import "` + depAddr.String() + `" "f" (func $f)) (func (export "main")))`
		if err := setContract(txn, ngtypes.NewContract(addr, mustWat(depWat), nil)); err != nil {
			return err
		}
		if err := checkActivate(txn, activate(t)); err == nil {
			t.Fatal("unknown dep must be refused")
		}

		// the dependency exists but is not active
		depWatSrc := `(module (func (export "f")))`
		if err := setContract(txn, ngtypes.NewContract(depAddr, mustWat(depWatSrc), nil)); err != nil {
			return err
		}
		if err := checkActivate(txn, activate(t)); !errors.Is(err, ErrDepNotActive) {
			t.Fatalf("inactive dep: got %v", err)
		}

		// the dependency activates: the check passes
		dep, err := getContract(txn, depAddr)
		if err != nil {
			return err
		}
		dep.SetActive(true)
		if err := setContract(txn, dep); err != nil {
			return err
		}
		if err := checkActivate(txn, activate(t)); err != nil {
			t.Fatalf("activate with a live dep: %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
