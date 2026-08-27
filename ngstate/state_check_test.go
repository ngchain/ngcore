package ngstate

import (
	"errors"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// effectSalt is the fixed reveal nonce every test reveal uses; a single salt
// keeps the derived commitment hash reproducible across a test's helpers.
var effectSalt = []byte("ngstate-test-salt")

// isEffectTx reports whether a tx type must go through commit-reveal
func isEffectTx(txType ngtypes.TxType) bool {
	switch txType {
	case ngtypes.TransactTx, ngtypes.DeployTx:
		return true
	default:
		return false
	}
}

// destroyExtra is the commit extra of a destroy: a deploy carrying EMPTY
// code. Applied to a live slot whose code exports `upgrade`, it removes the
// contract; on a slotless address it fails with ErrNothingToDestroy.
func destroyExtra() []byte {
	return ngtypes.EncodeCommitCode(nil)
}

// signedTx builds and signs a tx of the given type from priv. Effect txs
// (Transact/Deploy) are salted so they can be revealed; seed their
// commitment on chain with seedCommit before applying/checking them.
func signedTx(t *testing.T, priv *ngtypes.PrivateKey, txType ngtypes.TxType, to ngtypes.Address,
	value, fee *big.Int, extra []byte,
) *ngtypes.FullTx {
	t.Helper()

	tx := ngtypes.NewTx(ngtypes.ZERONET, txType, 1, to, value, fee, extra, nil)
	if isEffectTx(txType) {
		tx.Salt = effectSalt
	}
	if err := tx.Signature(priv); err != nil {
		t.Fatal(err)
	}
	return tx
}

// seedCommit records the commitment an effect tx reveals, at height 0 (in
// window and strictly earlier than the reveal height 1), so checkReveal and
// consumeReveal find it. A no-op for non-effect txs.
func seedCommit(t *testing.T, txn *bbolt.Tx, priv *ngtypes.PrivateKey, tx *ngtypes.FullTx) {
	t.Helper()

	if !isEffectTx(tx.Type) {
		return
	}

	from := ngtypes.NewAddress(priv)
	if err := putCommit(txn, 0, revealHash(tx), from); err != nil {
		t.Fatalf("seedCommit: %v", err)
	}
}

// activeContract builds a live (deployed) contract slot for source, the
// only contract state a caller ever sees now that there is no inactive step
func activeContract(addr ngtypes.Address, source []byte) *ngtypes.Contract {
	acc := ngtypes.NewContract(addr, source, nil)
	acc.SetActive(true)
	return acc
}

// activateInPlace brings an already-stored contract slot live and pins its
// dependencies, mirroring the dep bookkeeping handleDeploy performs — but
// on the slot as stored, so a pre-seeded context survives. Tests that seed
// a contract's kv up front (fixtures) use this instead of a full deploy tx,
// which would open a fresh empty slot.
func activateInPlace(t *testing.T, txn *bbolt.Tx, state *State, addr ngtypes.Address) {
	t.Helper()

	slot, err := getContract(txn, addr)
	if err != nil {
		t.Fatalf("activateInPlace: %v", err)
	}

	deps, err := validateDeps(txn, addr, slot.Source)
	if err != nil {
		t.Fatalf("activateInPlace deps: %v", err)
	}
	if err := pinDeps(txn, state.cs, deps); err != nil {
		t.Fatalf("activateInPlace pin: %v", err)
	}

	slot.SetActive(true)
	if err := setContractDeps(slot, deps); err != nil {
		t.Fatalf("activateInPlace setdeps: %v", err)
	}
	if err := setContract(txn, state.cs, slot); err != nil {
		t.Fatalf("activateInPlace store: %v", err)
	}
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
		// fund enough to afford the base-fee-adequate fee (value + fee) the valid
		// transact below pays now that the fee market is active from genesis
		if err := setBalance(txn, nil, userAddr, big.NewInt(1e18)); err != nil {
			return err
		}

		// a minimal block shell: CheckBlockTxs only consults the height
		// and the txs (never mutate the shared genesis block singleton)
		blockAt := func(height uint64, txs ...*ngtypes.FullTx) *ngtypes.FullBlock {
			genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
			header := *genesis.BlockHeader
			header.Height = height
			header.Coinbase = minerAddr[:] // the miner generate must pay this
			return &ngtypes.FullBlock{BlockHeader: &header, Txs: txs}
		}

		// a valid generate plus a valid transact
		gen := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
			minerAddr, ngtypes.GetBlockReward(1), big.NewInt(0), nil, nil)
		if err := gen.Signature(minerPriv); err != nil {
			return err
		}
		// fee must clear the genesis base fee (MinBaseFee * len(rlp(tx))); the
		// fork is active from height 0 now, so 1e15 comfortably covers a ~200B tx
		pay := signedTx(t, userPriv, ngtypes.TransactTx, minerAddr, big.NewInt(1), big.NewInt(1e15), nil)
		seedCommit(t, txn, userPriv, pay)

		if err := CheckBlockTxs(txn, blockAt(1, gen, pay)); err != nil {
			t.Fatalf("valid block refused: %v", err)
		}

		// an unsigned (non-generate) tx is refused
		unsigned := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.TransactTx, 1,
			minerAddr, big.NewInt(1), big.NewInt(0), nil, nil)
		if err := CheckBlockTxs(txn, blockAt(1, gen, unsigned)); !errors.Is(err, ngtypes.ErrTxUnsigned) {
			t.Fatalf("unsigned tx: got %v, want ErrTxUnsigned", err)
		}

		// an oversized extra is refused (the extra-size gate precedes the
		// reveal check in CheckBlockTxs, so no commitment is needed)
		fat := signedTx(t, userPriv, ngtypes.TransactTx, minerAddr,
			big.NewInt(1), big.NewInt(0), make([]byte, ngtypes.TxMaxExtraSize+1))
		if err := CheckBlockTxs(txn, blockAt(1, gen, fat)); !errors.Is(err, ngtypes.ErrTxExtraExcess) {
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
		if err := setBalance(txn, nil, addr, big.NewInt(100)); err != nil {
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

		// an effect tx with no committed commitment is refused as a non-reveal
		uncommitted := signedTx(t, priv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(1), big.NewInt(1), nil)
		if err := CheckTx(txn, uncommitted); !errors.Is(err, ErrTxNotCommitted) {
			t.Fatalf("uncommitted reveal: got %v, want ErrTxNotCommitted", err)
		}

		// transact: fine when funded, refused when the balance misses
		pay := signedTx(t, priv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(1), big.NewInt(1), nil)
		seedCommit(t, txn, priv, pay)
		if err := CheckTx(txn, pay); err != nil {
			t.Fatalf("funded transact: %v", err)
		}
		broke := signedTx(t, poorPriv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(1), big.NewInt(1), nil)
		seedCommit(t, txn, poorPriv, broke)
		if err := CheckTx(txn, broke); !errors.Is(err, ErrTxrBalanceInsufficient) {
			t.Fatalf("unfunded transact: got %v", err)
		}

		// destroy is an empty-code deploy: no slot -> refused as nothing to
		// destroy
		destroy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1), destroyExtra())
		seedCommit(t, txn, priv, destroy)
		if err := CheckTx(txn, destroy); !errors.Is(err, ErrNothingToDestroy) {
			t.Fatalf("destroy without a slot: got %v, want ErrNothingToDestroy", err)
		}

		// deploy onto the empty slot checks out
		deploy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(logWat)))
		seedCommit(t, txn, priv, deploy)
		if err := CheckTx(txn, deploy); err != nil {
			t.Fatalf("deploy: %v", err)
		}
		// a deploy whose extra is not a deploy payload is refused
		badDeploy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1), []byte{0xff})
		seedCommit(t, txn, priv, badDeploy)
		if err := CheckTx(txn, badDeploy); err == nil {
			t.Fatal("a broken deploy extra must be refused")
		}

		// open a LIVE slot whose code exports no `upgrade` hook: it is
		// immutable AND indestructible — the empty-code destroy is refused
		if err := setContract(txn, nil, activeContract(addr, mustWat(immutableWat))); err != nil {
			return err
		}
		if err := CheckTx(txn, destroy); !errors.Is(err, ErrImmutable) {
			t.Fatalf("destroy of an immutable contract: got %v, want ErrImmutable", err)
		}
		// re-deploying that immutable contract is refused for the same reason
		if err := CheckTx(txn, deploy); !errors.Is(err, ErrImmutable) {
			t.Fatalf("upgrade of an immutable contract: got %v, want ErrImmutable", err)
		}

		// swap in a LIVE slot whose code DOES export `upgrade`: the destroy
		// now checks out (there is no inactive state)
		if err := setContract(txn, nil, activeContract(addr, mustWat(upgradeableWat))); err != nil {
			return err
		}
		if err := CheckTx(txn, destroy); err != nil {
			t.Fatalf("destroy on an upgradeable live slot: %v", err)
		}

		// a referenced slot cannot be destroyed
		slot, err := getContract(txn, addr)
		if err != nil {
			return err
		}
		setRefCount(slot, 2)
		if err := setContract(txn, nil, slot); err != nil {
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

// TestCheckTxRejectsGenerate: CheckTx must REJECT a stray generate tx (e.g.
// one gossiped into the pool), not panic — a peer must not be able to crash
// the node by broadcasting a generate.
func TestCheckTxRejectsGenerate(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	gen := signedTx(t, priv, ngtypes.GenerateTx, ngtypes.NewAddress(priv),
		ngtypes.GetBlockReward(1), big.NewInt(0), nil)

	err := db.View(func(txn *bbolt.Tx) error {
		if err := CheckTx(txn, gen); !errors.Is(err, ngtypes.ErrTxTypeInvalid) {
			t.Errorf("CheckTx(generate) = %v, want ErrTxTypeInvalid (no panic)", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCheckDeployDeps covers the dependency validation at deploy time:
// self-deps, unknown deps and inactive deps are all refused. The module
// under validation is the one carried in the deploy tx's Extra
func TestCheckDeployDeps(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)
	depPriv, _ := ngtypes.GenerateKey()
	depAddr := ngtypes.NewAddress(depPriv)

	deploy := func(t *testing.T, source []byte) *ngtypes.FullTx {
		return signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(source))
	}

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, addr, big.NewInt(100)); err != nil {
			return err
		}

		// a module importing its OWN address
		selfWat := `(module (import "` + addr.String() + `" "f" (func $f)) (func (export "ng:main")))`
		if err := checkDeploy(txn, deploy(t, mustWat(selfWat))); !errors.Is(err, ErrDepSelf) {
			t.Fatalf("self dep: got %v", err)
		}

		// an unknown dependency address
		depWat := `(module (import "` + depAddr.String() + `" "f" (func $f)) (func (export "ng:main")))`
		if err := checkDeploy(txn, deploy(t, mustWat(depWat))); err == nil {
			t.Fatal("unknown dep must be refused")
		}

		// the dependency exists but is not active
		depWatSrc := `(module (func (export "f")))`
		if err := setContract(txn, nil, ngtypes.NewContract(depAddr, mustWat(depWatSrc), nil)); err != nil {
			return err
		}
		if err := checkDeploy(txn, deploy(t, mustWat(depWat))); !errors.Is(err, ErrDepNotActive) {
			t.Fatalf("inactive dep: got %v", err)
		}

		// the dependency goes live: the check passes
		dep, err := getContract(txn, depAddr)
		if err != nil {
			return err
		}
		dep.SetActive(true)
		if err := setContract(txn, nil, dep); err != nil {
			return err
		}
		if err := checkDeploy(txn, deploy(t, mustWat(depWat))); err != nil {
			t.Fatalf("deploy with a live dep: %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
