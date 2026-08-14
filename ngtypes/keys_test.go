package ngtypes

import (
	"math/big"
	"testing"
)

func testTx(participant Address) *FullTx {
	return NewUnsignedTx(ZERONET, TransactTx, 1, 1,
		[]Address{participant}, []*big.Int{big.NewInt(1)},
		big.NewInt(0), nil)
}

// TestTxSignatureRoundTrip: a single-key tx must verify against its
// own address and against nothing else
func TestTxSignatureRoundTrip(t *testing.T) {
	key, _ := GenerateKey()
	other, _ := GenerateKey()

	tx := testTx(NewAddress(key))
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(NewAddress(key)); err != nil {
		t.Fatalf("own-address verify: %v", err)
	}
	if err := tx.Verify(NewAddress(other)); err == nil {
		t.Fatal("a foreign address must not verify")
	}

	tx.Extra = []byte("tampered")
	if err := tx.Verify(NewAddress(key)); err == nil {
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
	if err := tx.Verify(NewAddress(key)); err != nil {
		t.Fatalf("restored key cannot spend: %v", err)
	}

	// a wrong-length secret must be rejected
	if _, err := ParsePrivateKey(key.Serialize()[1:]); err == nil {
		t.Fatal("a truncated seed must not parse")
	}
}
