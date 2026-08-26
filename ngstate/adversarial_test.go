package ngstate

import (
	"errors"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// shadowHookWat is an adversary's (or a careless developer's) contract that
// exports the reserved hook names WITHOUT the ng: namespace. If the protocol
// matched hooks by bare name, this `validate` would silently arm the
// account-abstraction gate (and trap every tx) and this `upgrade` would make
// the contract mutable. Under the ng: namespace neither is a protocol hook —
// they are ordinary, inert exports.
const shadowHookWat = `(module
	(func (export "validate") unreachable)
	(func (export "upgrade"))
	(func (export "ng:main")))`

// TestNamespaceShadowingNotArmed proves the ng: namespace defense: a contract
// exporting a plain (un-namespaced) `validate` does NOT gate the account's
// txs, so a name collision cannot accidentally — or maliciously — arm the AA
// hook. Contrast TestValidateGateVetoes, where the SAME trapping body under
// `ng:validate` does veto every tx.
func TestNamespaceShadowingNotArmed(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		gen := signedTx(t, priv, ngtypes.GenerateTx, addr, big.NewInt(1000), big.NewInt(0), nil)
		deploy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(shadowHookWat)))
		seedCommit(t, txn, priv, deploy)
		if err := state.HandleTxs(txn, 1, gen, deploy); err != nil {
			t.Fatalf("deploy the shadow-hook contract: %v", err)
		}

		// a plain outbound tx: if `validate` were armed it would trap and the
		// tx would fail ErrTxUnauthorized. It must instead succeed and move value.
		pay := signedTx(t, priv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(5), big.NewInt(1), nil)
		seedCommit(t, txn, priv, pay)
		if err := state.HandleTxs(txn, 1, pay); err != nil {
			t.Fatalf("plain `validate` export must NOT gate the tx, got: %v", err)
		}
		if getBalance(txn, testAddr(0x01)).Cmp(big.NewInt(5)) != 0 {
			t.Fatal("the transfer did not move value")
		}

		// the un-namespaced `upgrade` must likewise NOT authorize a destroy:
		// with no ng:upgrade hook the contract is immutable
		destroy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1), destroyExtra())
		if err := checkDeploy(txn, destroy); !errors.Is(err, ErrImmutable) {
			t.Fatalf("plain `upgrade` export must not make the contract mutable, got: %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCommitDuplicateRejected proves the anti-double-charge guard: because a
// commitment now signs a height-independent digest, an attacker could re-height
// a gossiped commitment to get it included — and its fee charged — twice. A
// commitment hash already pending on chain makes any second inclusion invalid,
// so the committer is charged at most once.
func TestCommitDuplicateRejected(t *testing.T) {
	db := newTestDB(t)

	priv, _ := ngtypes.GenerateKey()
	from := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, from, big.NewInt(1000)); err != nil {
			return err
		}

		hash := make([]byte, ngtypes.HashSize)
		hash[0] = 0xd0
		// a commitment already recorded at height 5
		if err := putCommit(txn, 5, hash, from); err != nil {
			return err
		}

		// the SAME hash re-heighted to 6 (a re-heighted duplicate) is rejected,
		// at both the pool gate and — via commitHashPending — the block-apply gate
		dup := ngtypes.NewCommitment(ngtypes.ZERONET, 6, hash, big.NewInt(100))
		if err := dup.Signature(priv); err != nil {
			return err
		}
		if err := CheckCommitment(txn, dup, 6); !errors.Is(err, ErrCommitDuplicate) {
			t.Fatalf("re-heighted duplicate commit: got %v, want ErrCommitDuplicate", err)
		}
		if !commitHashPending(txn, hash, 6) {
			t.Fatal("the pending commitment must be detected as a duplicate")
		}

		// a DIFFERENT hash is of course fine
		other := ngtypes.NewCommitment(ngtypes.ZERONET, 6, make([]byte, ngtypes.HashSize), big.NewInt(100))
		if err := other.Signature(priv); err != nil {
			return err
		}
		if err := CheckCommitment(txn, other, 6); err != nil {
			t.Fatalf("a distinct commitment must be admissible: %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCommitmentSingleUseNoDoubleSpend proves that height-flexible reveal
// signatures do NOT enable a double-spend: one commitment funds exactly ONE
// reveal. After a reveal consumes the commitment, a second reveal of the same
// content at a different (still in-window) height — trivial to build now that
// the signature is height-independent — is rejected because the commitment is
// already spent.
func TestCommitmentSingleUseNoDoubleSpend(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, _ := ngtypes.GenerateKey()
	from := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		if err := setBalance(txn, nil, from, big.NewInt(1000)); err != nil {
			return err
		}
		// one commitment at height 5, binding the shared reveal content
		if err := putCommit(txn, 5, revealHash(revealAt(t, priv, 6)), from); err != nil {
			return err
		}

		// the SAME signature verifies at 6 and 7 (height-independent), so both
		// reveals are individually well-formed
		r1 := revealAt(t, priv, 6)
		r2 := revealAt(t, priv, 7)
		if err := CheckTx(txn, r1); err != nil {
			t.Fatalf("first reveal must pass CheckTx: %v", err)
		}
		if err := CheckTx(txn, r2); err != nil {
			t.Fatalf("second reveal is also well-formed pre-consume: %v", err)
		}

		// consume the commitment with the first reveal
		if err := state.consumeReveal(txn, r1); err != nil {
			t.Fatalf("consume first reveal: %v", err)
		}

		// the second reveal now finds no unspent commitment — no double-spend,
		// at either the pool gate or the block-apply gate
		if err := CheckTx(txn, r2); !errors.Is(err, ErrTxNotCommitted) {
			t.Fatalf("double-spend via a second reveal: got %v, want ErrTxNotCommitted", err)
		}
		if err := state.consumeReveal(txn, r2); !errors.Is(err, ErrTxNotCommitted) {
			t.Fatalf("consumeReveal must refuse the double-spend: got %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
