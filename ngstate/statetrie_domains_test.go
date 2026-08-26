package ngstate

import (
	"bytes"
	"math/big"
	"testing"

	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/statetrie"
	"github.com/ngchain/ngcore/utils"
)

// TestStateProofAllDomains rounds out the light-client coverage: beyond the
// balance domain, a Merkle proof of a CONTRACT slot, a registered public KEY,
// and a pending COMMITMENT each verifies against the same state root, and a
// tampered proof is rejected. This exercises every branch of stateDomain and
// the per-domain leaf encoding end to end.
func TestStateProofAllDomains(t *testing.T) {
	db := newTestDB(t)

	// a post-quantum key so its full-envelope signature registers the pubkey
	// on chain (recovery-scheme keys are never stored, so have no key leaf)
	priv, err := ngtypes.GenerateSchemeKey(ngtypes.SchemeMLDSA44)
	if err != nil {
		t.Fatal(err)
	}
	signer := ngtypes.NewAddress(priv)

	contractAddr := testAddr(0xc0)
	commitHash := utils.Hash256([]byte("a-pending-commitment"))
	commitFrom := testAddr(0xf0)

	err = db.Update(func(txn *bbolt.Tx) error {
		// contract domain: a live contract slot
		if err := setContract(txn, nil, activeContract(contractAddr, mustWat(`(module (func (export "ng:main")))`))); err != nil {
			return err
		}
		// key domain: register the signer's pubkey via the real choke point
		regTx := signedTx(t, priv, ngtypes.TransactTx, testAddr(0x01), big.NewInt(0), big.NewInt(1), nil)
		if err := registerPubKey(txn, nil, regTx); err != nil {
			return err
		}
		// commit domain: a pending commitment at height 5
		if err := putCommit(txn, 5, commitHash, commitFrom); err != nil {
			return err
		}

		cases := []struct {
			domain string
			rawKey []byte
		}{
			{"contract", contractAddr[:]},
			{"key", signer[:]},
			{"commit", commitKey(5, commitHash)},
		}
		for _, c := range cases {
			root, path, value, valueHash, proof, err := StateProof(txn, c.domain, c.rawKey)
			if err != nil {
				t.Fatalf("%s: StateProof: %v", c.domain, err)
			}
			if len(value) == 0 {
				t.Fatalf("%s: proof has no value (leaf absent)", c.domain)
			}
			if !bytes.Equal(valueHash, statetrie.ValueHash(value)) {
				t.Fatalf("%s: valueHash does not match the stored value", c.domain)
			}
			if !statetrie.Verify(root, path, valueHash, proof) {
				t.Fatalf("%s: inclusion proof rejected by Verify", c.domain)
			}
			// every domain's leaf must root into the SAME state root
			if !bytes.Equal(root, StateRoot(txn)) {
				t.Fatalf("%s: proof root diverges from the state root", c.domain)
			}
			// tamper a sibling -> rejected
			if len(proof) > 0 {
				bad := append([][]byte{}, proof...)
				bad[0] = append([]byte{}, bad[0]...)
				bad[0][0] ^= 0xff
				if statetrie.Verify(root, path, valueHash, bad) {
					t.Fatalf("%s: tampered proof accepted", c.domain)
				}
			}
		}

		// commit domain absence: a hash that was never committed
		gone := commitKey(5, utils.Hash256([]byte("never-committed")))
		r, p, v, vh, pr, err := StateProof(txn, "commit", gone)
		if err != nil {
			return err
		}
		if len(v) != 0 {
			t.Fatalf("commit absence proof carries a value %x", v)
		}
		if !statetrie.Verify(r, p, vh, pr) {
			t.Fatal("commit absence proof rejected")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
