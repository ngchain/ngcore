package ngtypes

import (
	"bytes"
	"math/big"
	"testing"
)

// fixed seeds keep the pq keygen deterministic and the test fast
func seededKey(t *testing.T, scheme SigScheme, fill byte) *PrivateKey {
	t.Helper()

	seed := bytes.Repeat([]byte{fill}, KeySeedSize)
	key, err := NewKeyFromSeed(scheme, seed)
	if err != nil {
		t.Fatal(err)
	}

	return key
}

// registryOf resolves compact envelopes for the given keys, like the
// on-chain key registry would
func registryOf(keys ...*PrivateKey) PubKeyResolver {
	reg := make(map[Address][]byte, len(keys))
	for _, key := range keys {
		entry := append([]byte{byte(key.Scheme)}, key.PublicBytes()...)
		reg[NewAddress(key)] = entry
	}

	return func(addr Address) []byte {
		return reg[addr]
	}
}

// TestCompactEnvelope: a compact-signed tx verifies through the key
// registry, resolves its From, and fails without a registry
func TestCompactEnvelope(t *testing.T) {
	key := seededKey(t, SchemeMLDSA44, 0x01)

	tx := testTx(Address{})
	if err := tx.SignatureCompact(key); err != nil {
		t.Fatal(err)
	}

	if !tx.IsCompactEnvelope() {
		t.Fatal("SignatureCompact must produce the compact form")
	}
	if tx.EnvelopeScheme() != SchemeMLDSA44 {
		t.Fatalf("envelope scheme = %#02x", byte(tx.EnvelopeScheme()))
	}
	if want := 2 + AddressSize + SigSize(SchemeMLDSA44); len(tx.Sign) != want {
		t.Fatalf("compact envelope = %d bytes, want %d", len(tx.Sign), want)
	}

	if from, err := tx.From(); err != nil || !from.Equals(NewAddress(key)) {
		t.Fatalf("compact From = %v, %v", from, err)
	}

	if err := tx.Verify(registryOf(key)); err != nil {
		t.Fatalf("compact verify with registry: %v", err)
	}

	// no registry at all
	if err := tx.Verify(nil); err == nil {
		t.Fatal("compact envelope must not verify without a registry")
	}

	// an empty registry
	if err := tx.Verify(registryOf()); err == nil {
		t.Fatal("compact envelope must not verify for an unregistered address")
	}

	// a poisoned registry: right length, wrong key for the address
	other := seededKey(t, SchemeMLDSA44, 0x02)
	poisoned := func(Address) []byte {
		return append([]byte{byte(SchemeMLDSA44)}, other.PublicBytes()...)
	}
	if err := tx.Verify(poisoned); err == nil {
		t.Fatal("a registry entry not hashing to the address must be rejected")
	}
}

// TestCompactEnvelopeRecoverySchemes: secp keys have no compact form —
// they degrade to the even smaller recover envelope
func TestCompactEnvelopeRecoverySchemes(t *testing.T) {
	key := seededKey(t, SchemeSecp256k1, 0x03)

	tx := testTx(Address{})
	if err := tx.SignatureCompact(key); err != nil {
		t.Fatal(err)
	}

	if tx.IsCompactEnvelope() {
		t.Fatal("a recovery scheme must not use the compact form")
	}
	if want := 2 + SigSize(SchemeSecp256k1); len(tx.Sign) != want {
		t.Fatalf("recover envelope = %d bytes, want %d", len(tx.Sign), want)
	}
	if err := tx.Verify(nil); err != nil {
		t.Fatalf("recover envelope verify: %v", err)
	}
}

// TestEnvelopeMalformed drives every reject branch of the envelope
// parser through Verify and From
func TestEnvelopeMalformed(t *testing.T) {
	key := seededKey(t, SchemeSecp256k1, 0x04)
	sigLen := SigSize(SchemeSecp256k1)

	cases := map[string][]byte{
		"empty":                    {},
		"one byte":                 {envelopeFull},
		"unknown scheme":           {envelopeFull, 0x99, 0x00},
		"unknown tag":              append([]byte{0x77, byte(SchemeSecp256k1)}, make([]byte, sigLen)...),
		"full wrong length":        {envelopeFull, byte(SchemeSecp256k1), 0x00},
		"compact wrong length":     {envelopeCompact, byte(SchemeMLDSA44), 0x00},
		"recover wrong length":     {envelopeRecover, byte(SchemeSecp256k1), 0x00},
		"recover without recovery": append([]byte{envelopeRecover, byte(SchemeMLDSA44)}, make([]byte, SigSize(SchemeMLDSA44))...),
		"recover garbage sig":      append([]byte{envelopeRecover, byte(SchemeSecp256k1)}, make([]byte, sigLen)...),
	}

	for name, sign := range cases {
		tx := testTx(NewAddress(key))
		tx.ManuallySetSignature(sign)

		if err := tx.Verify(nil); err == nil {
			t.Fatalf("%s: Verify must fail", name)
		}
		if _, err := tx.From(); err == nil {
			t.Fatalf("%s: From must fail", name)
		}
	}

	// a full envelope with the right shape but a corrupted signature
	tx := testTx(NewAddress(key))
	mlKey := seededKey(t, SchemeMLDSA44, 0x05)
	if err := tx.Signature(mlKey); err != nil {
		t.Fatal(err)
	}
	tx.Sign[len(tx.Sign)-1] ^= 0xff
	if err := tx.Verify(nil); err == nil {
		t.Fatal("a corrupted signature must not verify")
	}

	// EnvelopeScheme on an unsigned tx
	unsigned := testTx(Address{})
	if unsigned.EnvelopeScheme() != 0 {
		t.Fatal("an unsigned tx has no scheme")
	}
	if unsigned.IsCompactEnvelope() {
		t.Fatal("an unsigned tx has no compact envelope")
	}
}

// TestFromMalformed drives the From-specific reject branches that the
// envelope parser shares but From re-validates on its own
func TestFromMalformed(t *testing.T) {
	// full envelope, unknown scheme (pkLen == 0)
	tx := testTx(Address{})
	tx.ManuallySetSignature([]byte{envelopeFull, 0x99, 0x00})
	if _, err := tx.From(); err == nil {
		t.Fatal("unknown scheme must fail From")
	}

	// compact envelope, unknown scheme
	tx.ManuallySetSignature([]byte{envelopeCompact, 0x99, 0x00})
	if _, err := tx.From(); err == nil {
		t.Fatal("unknown compact scheme must fail From")
	}

	// unsigned
	tx.ManuallySetSignature(nil)
	if _, err := tx.From(); err == nil {
		t.Fatal("unsigned From must fail")
	}
}

// TestSignatureCompactValueTransfer exercises a full compact spend:
// value/fee carried, envelope verified against the registry
func TestSignatureCompactValueTransfer(t *testing.T) {
	key := seededKey(t, SchemeFNDSA512, 0x06)

	tx := NewUnsignedTx(ZERONET, TransactTx, 5, Address{7}, big.NewInt(42), big.NewInt(1), nil)
	if err := tx.SignatureCompact(key); err != nil {
		t.Fatal(err)
	}
	if err := tx.CheckTransaction(registryOf(key)); err != nil {
		t.Fatalf("compact transact: %v", err)
	}
}
