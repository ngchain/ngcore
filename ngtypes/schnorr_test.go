package ngtypes

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// TestBIP340Vector pins the signature scheme to the FINAL bip-340
// standard via its official test vector 0 — the previous library
// implemented the 2019 draft, whose signatures are incompatible
func TestBIP340Vector(t *testing.T) {
	pubKeyBytes, _ := hex.DecodeString("F9308A019258C31049344F85F89D5229B531C845836F99B08601F113BCE036F9")
	msg, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000000")
	sigBytes, _ := hex.DecodeString("E907831F80848D1069A5371B402410364BDF1C5F8307B0084C55F1CE2DCA8215" +
		"25F66A4A85EA8B71E482A74F382D2CE5EBEEE8FDB2172F477DF4900D310536C0")

	pub, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		t.Fatal(err)
	}

	if !sig.Verify(msg, pub) {
		t.Fatal("the official bip-340 vector 0 must verify")
	}

	// flipping one bit must break it
	broken := make([]byte, len(sigBytes))
	copy(broken, sigBytes)
	broken[63] ^= 1
	if badSig, err := schnorr.ParseSignature(broken); err == nil && badSig.Verify(msg, pub) {
		t.Fatal("a corrupted signature must not verify")
	}
}

// TestTxSignatureRoundTrip: txs signed by one key and by sharded keys
// must verify against their addresses' public keys, and against
// nothing else
func TestTxSignatureRoundTrip(t *testing.T) {
	key1, _ := btcec.NewPrivateKey()
	key2, _ := btcec.NewPrivateKey()

	newTx := func() *FullTx {
		return NewUnsignedTx(ZERONET, TransactTx, 1, 1,
			[]Address{NewAddress(key1)}, []*big.Int{big.NewInt(1)},
			big.NewInt(0), nil)
	}

	// single key
	tx := newTx()
	if err := tx.Signature(key1); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(NewAddress(key1).PubKey()); err != nil {
		t.Fatalf("own-key verify: %v", err)
	}
	if err := tx.Verify(NewAddress(key2).PubKey()); err == nil {
		t.Fatal("a foreign key must not verify the signature")
	}

	// sharded owner: the multi-key address is the pubkey of the scalar
	// sum, and signing with all shards produces a plain bip-340 sig
	multiAddr, err := NewAddressFromMultiKeys(key1, key2)
	if err != nil {
		t.Fatal(err)
	}

	tx = newTx()
	if err := tx.Signature(key1, key2); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(multiAddr.PubKey()); err != nil {
		t.Fatalf("multi-key verify: %v", err)
	}
	if err := tx.Verify(NewAddress(key1).PubKey()); err == nil {
		t.Fatal("a single shard must not verify the combined signature")
	}

	// tampering with the payload must invalidate the signature
	tx.Extra = []byte("tampered")
	if err := tx.Verify(multiAddr.PubKey()); err == nil {
		t.Fatal("a tampered tx must not verify")
	}
}
