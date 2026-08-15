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

// TestTxSignatureRoundTrip: a signed tx verifies, derives its From address
// from the embedded key, and any tampering invalidates it
func TestTxSignatureRoundTrip(t *testing.T) {
	key, _ := GenerateKey()
	other, _ := GenerateKey()

	tx := testTx(NewAddress(other))
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(nil); err != nil {
		t.Fatalf("verify: %v", err)
	}

	from, err := tx.From()
	if err != nil {
		t.Fatal(err)
	}
	if !from.Equals(NewAddress(key)) {
		t.Fatal("the From address must derive from the signer's key")
	}
	if from.Equals(NewAddress(other)) {
		t.Fatal("the From address must not equal a foreign address")
	}

	tx.Extra = []byte("tampered")
	if err := tx.Verify(nil); err == nil {
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
	if err := tx.Verify(nil); err != nil {
		t.Fatalf("restored key cannot spend: %v", err)
	}
	if from, _ := tx.From(); !from.Equals(NewAddress(key)) {
		t.Fatal("restored key derives a different from")
	}

	// a wrong-length secret must be rejected
	if _, err := ParsePrivateKey(key.Serialize()[1:]); err == nil {
		t.Fatal("a truncated seed must not parse")
	}
}

// TestAllSchemes: every scheme on the menu signs, verifies, round
// trips its wallet seed, and derives scheme-bound addresses
func TestAllSchemes(t *testing.T) {
	schemes := []SigScheme{SchemeFNDSA512, SchemeMLDSA44, SchemeSLHDSA128}

	for _, scheme := range schemes {
		key, err := GenerateSchemeKey(scheme)
		if err != nil {
			t.Fatalf("scheme %#02x: %v", byte(scheme), err)
		}

		// wallet round trip
		restored, err := ParsePrivateKey(key.Serialize())
		if err != nil {
			t.Fatal(err)
		}
		if !NewAddress(restored).Equals(NewAddress(key)) {
			t.Fatalf("scheme %#02x: seed round trip changed the address", byte(scheme))
		}

		// sign + verify through the tx envelope
		tx := testTx(NewAddress(key))
		if err := tx.Signature(key); err != nil {
			t.Fatalf("scheme %#02x sign: %v", byte(scheme), err)
		}
		wantLen := 2 + PubKeySize(scheme) + SigSize(scheme)
		if len(tx.Sign) != wantLen {
			t.Fatalf("scheme %#02x envelope = %d bytes, want %d", byte(scheme), len(tx.Sign), wantLen)
		}
		if err := tx.Verify(nil); err != nil {
			t.Fatalf("scheme %#02x verify: %v", byte(scheme), err)
		}
		if from, _ := tx.From(); !from.Equals(NewAddress(key)) {
			t.Fatalf("scheme %#02x: wrong derived sender", byte(scheme))
		}

		tx.Extra = []byte("tampered")
		if err := tx.Verify(nil); err == nil {
			t.Fatalf("scheme %#02x: tampered tx must not verify", byte(scheme))
		}
	}

	// the same seed under different schemes gives DIFFERENT addresses:
	// the scheme byte binds into the address preimage
	seed := make([]byte, KeySeedSize)
	a, _ := NewKeyFromSeed(SchemeFNDSA512, seed)
	b, _ := NewKeyFromSeed(SchemeMLDSA44, seed)
	if NewAddress(a).Equals(NewAddress(b)) {
		t.Fatal("addresses must be scheme-separated")
	}
}
