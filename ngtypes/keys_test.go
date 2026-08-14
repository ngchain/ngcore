package ngtypes

import (
	"math/big"
	"testing"
)

func testTx(to Address) *FullTx {
	return NewUnsignedTx(ZERONET, TransactTx, 1,
		to, big.NewInt(1),
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
