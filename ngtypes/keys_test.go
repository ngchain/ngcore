package ngtypes

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// TestBIP340Vector pins the scheme to the FINAL bip-340 standard via
// its official test vector 0
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
}

// TestCoSigning: the many-to-many primitive — several co-signers
// combine into one key; the tx they sign together verifies as a plain
// single signature whose sender is the shared address
func TestCoSigning(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()
	key3, _ := GenerateKey()

	shared, err := NewAddressFromMultiKeys(key1, key2, key3)
	if err != nil {
		t.Fatal(err)
	}

	tx := testTx(NewAddress(key1))
	if err := tx.Signature(key1, key2, key3); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(); err != nil {
		t.Fatalf("co-signed tx must verify: %v", err)
	}

	sender, err := tx.Sender()
	if err != nil {
		t.Fatal(err)
	}
	if !sender.Equals(shared) {
		t.Fatal("the sender must be the combined (shared) address")
	}

	// a subset of the co-signers cannot spend the shared address
	tx2 := testTx(NewAddress(key1))
	if err := tx2.Signature(key1, key2); err != nil {
		t.Fatal(err)
	}
	if sender2, _ := tx2.Sender(); sender2.Equals(shared) {
		t.Fatal("a subset must not derive the shared address")
	}

	// order does not matter: addition commutes
	altAddr, err := NewAddressFromMultiKeys(key3, key1, key2)
	if err != nil {
		t.Fatal(err)
	}
	if !altAddr.Equals(shared) {
		t.Fatal("the combined address must be order-independent")
	}
}

func testTx(participant Address) *FullTx {
	return NewUnsignedTx(ZERONET, TransactTx, 1,
		[]Address{participant}, []*big.Int{big.NewInt(1)},
		big.NewInt(0), nil)
}

// TestTxSignatureRoundTrip: a signed tx verifies, derives its sender
// from the embedded key, and any tampering invalidates it
func TestTxSignatureRoundTrip(t *testing.T) {
	key, _ := GenerateKey()
	other, _ := GenerateKey()

	tx := testTx(NewAddress(other))
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}

	sender, err := tx.Sender()
	if err != nil {
		t.Fatal(err)
	}
	if !sender.Equals(NewAddress(key)) {
		t.Fatal("the sender must derive from the signer's key")
	}
	if sender.Equals(NewAddress(other)) {
		t.Fatal("the sender must not equal a foreign address")
	}

	tx.Extra = []byte("tampered")
	if err := tx.Verify(); err == nil {
		t.Fatal("a tampered tx must not verify")
	}
}

// TestKeySerializeRoundTrip: the wallet format (scheme + 32-byte seed)
// restores the identical key and address
func TestKeySerializeRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	restored, err := ParsePrivateKey(key.Serialize())
	if err != nil {
		t.Fatal(err)
	}

	if !NewAddress(restored).Equals(NewAddress(key)) {
		t.Fatal("serialize round trip changed the address")
	}

	tx := testTx(NewAddress(key))
	if err := tx.Signature(restored); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(); err != nil {
		t.Fatalf("restored key cannot spend: %v", err)
	}
	if sender, _ := tx.Sender(); !sender.Equals(NewAddress(key)) {
		t.Fatal("restored key derives a different sender")
	}

	// a wrong-length secret must be rejected
	if _, err := ParsePrivateKey(key.Serialize()[1:]); err == nil {
		t.Fatal("a truncated seed must not parse")
	}
}
