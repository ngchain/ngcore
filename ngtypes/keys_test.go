package ngtypes

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// TestBIP340Vector pins the legacy scheme to the FINAL bip-340
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

func testTx(participant Address) *FullTx {
	return NewUnsignedTx(ZERONET, TransactTx, 1, 1,
		[]Address{participant}, []*big.Int{big.NewInt(1)},
		big.NewInt(0), nil)
}

// TestTxSignatureRoundTrip: single-key txs under both schemes must
// verify against their own address and against nothing else
func TestTxSignatureRoundTrip(t *testing.T) {
	pqKey, _ := GenerateKey()
	legacyKey, _ := GenerateLegacyKey()

	for name, key := range map[string]*PrivateKey{"mldsa44": pqKey, "secp-schnorr": legacyKey} {
		other, _ := GenerateKey()

		tx := testTx(NewAddress(key))
		if err := tx.Signature(key); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := tx.Verify(NewAddress(key)); err != nil {
			t.Fatalf("%s: own-address verify: %v", name, err)
		}
		if err := tx.Verify(NewAddress(other)); err == nil {
			t.Fatalf("%s: a foreign address must not verify", name)
		}

		tx.Extra = []byte("tampered")
		if err := tx.Verify(NewAddress(key)); err == nil {
			t.Fatalf("%s: a tampered tx must not verify", name)
		}
	}
}

// TestNativeMultisig: a 2-of-3 keyset mixing post-quantum and legacy
// keys — any two members can spend, one cannot, and the exact
// threshold rule forbids surplus signatures
func TestNativeMultisig(t *testing.T) {
	key1, _ := GenerateKey()       // ML-DSA-44
	key2, _ := GenerateLegacyKey() // secp schnorr: hybrid keyset
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
		t.Fatalf("2-of-3 (mldsa+secp) verify: %v", err)
	}

	// a different pair works too
	tx = testTx(addr)
	if err := tx.SignMultisig(2, members, key2, key3); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(addr); err != nil {
		t.Fatalf("2-of-3 (secp+mldsa) verify: %v", err)
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
// restores the identical key and address for both schemes
func TestKeySerializeRoundTrip(t *testing.T) {
	for _, gen := range []func() (*PrivateKey, error){GenerateKey, GenerateLegacyKey} {
		key, err := gen()
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
	}
}
