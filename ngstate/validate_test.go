package ngstate

import (
	"errors"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
)

// validateFreezeWat is a native account-abstraction policy that vetoes EVERY
// tx: a frozen account. The deploy that INSTALLS it is not gated (the slot is
// not yet live with `validate` when runValidate runs), but any later tx from
// the account is rejected as unauthorized.
const validateFreezeWat = `(module (func (export "ng:validate") unreachable))`

// validateAllowlistWat permits a tx only when its recipient's first address
// byte is 0xAA — a minimal allow-list, proving `validate` can inspect the tx
// through the tx.* host fns and selectively authorize.
const validateAllowlistWat = `(module
	(import "tx" "get_to" (func $to (param i32) (result i32)))
	(memory 1)
	(func (export "ng:validate")
		(drop (call $to (i32.const 0)))
		(if (i32.ne (i32.load8_u (i32.const 0)) (i32.const 0xaa))
			(then unreachable))))`

// TestValidateGateVetoes: once an account has a live contract exporting
// `validate` that rejects, every tx it sends is a HARD failure
// (ErrTxUnauthorized), and the block carrying it is invalid.
func TestValidateGateVetoes(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		gen := signedTx(t, priv, ngtypes.GenerateTx, addr, big.NewInt(1000), big.NewInt(0), nil)
		deploy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(validateFreezeWat)))
		seedCommit(t, txn, priv, deploy)

		// installing the validate hook is itself not gated
		if err := state.HandleTxs(txn, 1, gen, deploy); err != nil {
			t.Fatalf("installing the validate hook must not be gated: %v", err)
		}
		if !contractExists(txn, addr) {
			t.Fatal("the validate contract must be live")
		}

		// any subsequent tx from the account is now vetoed by its own policy
		pay := signedTx(t, priv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(5), big.NewInt(1), nil)
		seedCommit(t, txn, priv, pay)
		if err := state.HandleTxs(txn, 1, pay); !errors.Is(err, ErrTxUnauthorized) {
			t.Fatalf("frozen account tx: got %v, want ErrTxUnauthorized", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestValidateGateAllowlist: a `validate` hook selectively authorizes on the
// tx context — a disallowed recipient is vetoed, an allow-listed one passes
// and the value moves. An account with no `validate` export is unaffected.
func TestValidateGateAllowlist(t *testing.T) {
	db := newTestDB(t)
	state := &State{Network: ngtypes.ZERONET}

	priv, _ := ngtypes.GenerateKey()
	addr := ngtypes.NewAddress(priv)

	err := db.Update(func(txn *bbolt.Tx) error {
		gen := signedTx(t, priv, ngtypes.GenerateTx, addr, big.NewInt(1000), big.NewInt(0), nil)
		deploy := signedTx(t, priv, ngtypes.DeployTx, ngtypes.Address{}, nil, big.NewInt(1),
			ngtypes.EncodeCommitCode(mustWat(validateAllowlistWat)))
		seedCommit(t, txn, priv, deploy)
		if err := state.HandleTxs(txn, 1, gen, deploy); err != nil {
			t.Fatalf("deploy the allow-list policy: %v", err)
		}

		// a tx to a NON allow-listed recipient is vetoed
		bad := signedTx(t, priv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(5), big.NewInt(1), nil)
		seedCommit(t, txn, priv, bad)
		if err := state.HandleTxs(txn, 1, bad); !errors.Is(err, ErrTxUnauthorized) {
			t.Fatalf("disallowed recipient: got %v, want ErrTxUnauthorized", err)
		}

		// a tx to the allow-listed recipient (0xAA...) passes the policy
		good := signedTx(t, priv, ngtypes.TransactTx, testAddr(0xaa), big.NewInt(5), big.NewInt(1), nil)
		seedCommit(t, txn, priv, good)
		if err := state.HandleTxs(txn, 1, good); err != nil {
			t.Fatalf("allow-listed recipient must pass: %v", err)
		}
		if getBalance(txn, testAddr(0xaa)).Cmp(big.NewInt(5)) != 0 {
			t.Fatal("the authorized transfer did not move value")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
