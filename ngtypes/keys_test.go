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

// TestNativeMultisig: a 2-of-3 keyset — any two members can spend,
// one cannot, and the exact threshold rule forbids surplus signatures
func TestNativeMultisig(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()
	key3, _ := GenerateKey()

	addr, err := NewMultisigAddress(2, key1, key2, key3)
	if err != nil {
		t.Fatal(err)
	}

	members := []KeysetMember{
		{Scheme: key1.Scheme, PubKey: key1.PublicBytes()},
		{Scheme: key2.Scheme, PubKey: key2.PublicBytes()},
		{Scheme: key3.Scheme, PubKey: key3.PublicBytes()},
	}

	// two members of different schemes sign
	tx := testTx(addr)
	if err := tx.SignMultisig(2, members, key1, key2); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(addr); err != nil {
		t.Fatalf("2-of-3 (key1+key2) verify: %v", err)
	}

	// a different pair works too
	tx = testTx(addr)
	if err := tx.SignMultisig(2, members, key2, key3); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(addr); err != nil {
		t.Fatalf("2-of-3 (key2+key3) verify: %v", err)
	}

	// one signer is refused at signing time
	tx = testTx(addr)
	if err := tx.SignMultisig(2, members, key1); err == nil {
		t.Fatal("threshold 2 with one signer must fail")
	}

	// and a hand-crafted under-threshold envelope fails verification
	tx = testTx(addr)
	if err := tx.SignMultisig(1, members[:1], key1); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(addr); err == nil {
		t.Fatal("a 1-of-1 envelope must not satisfy a 2-of-3 address")
	}

	// an outsider cannot pose as a member
	outsider, _ := GenerateKey()
	tx = testTx(addr)
	if err := tx.SignMultisig(2, members, key1, outsider); err == nil {
		t.Fatal("an outsider signer must be rejected")
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

	// an unknown scheme byte must be rejected
	bad := key.Serialize()
	bad[0] = 0x7f
	if _, err := ParsePrivateKey(bad); err == nil {
		t.Fatal("an unknown scheme byte must not parse")
	}
}
