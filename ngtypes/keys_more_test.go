package ngtypes

import (
	"bytes"
	"testing"
)

const bogusScheme = SigScheme(0x99)

func TestSchemeSizes(t *testing.T) {
	for _, scheme := range []SigScheme{SchemeSecp256k1, SchemeFNDSA512, SchemeMLDSA44, SchemeSLHDSA128} {
		if PubKeySize(scheme) <= 0 || SigSize(scheme) <= 0 {
			t.Fatalf("scheme %#02x sizes must be positive", byte(scheme))
		}
	}

	if PubKeySize(bogusScheme) != 0 || SigSize(bogusScheme) != 0 {
		t.Fatal("unknown schemes have zero sizes")
	}
	if HasRecovery(SchemeMLDSA44) || HasRecovery(bogusScheme) {
		t.Fatal("only secp256k1 recovers")
	}
}

func TestNewKeyFromSeedErrors(t *testing.T) {
	if _, err := NewKeyFromSeed(SchemeSecp256k1, make([]byte, 5)); err == nil {
		t.Fatal("a short seed must be rejected")
	}
	if _, err := NewKeyFromSeed(bogusScheme, make([]byte, KeySeedSize)); err == nil {
		t.Fatal("an unknown scheme must be rejected")
	}
	if _, err := GenerateSchemeKey(bogusScheme); err == nil {
		t.Fatal("generating under an unknown scheme must fail")
	}

	// determinism: one seed, one key
	seed := bytes.Repeat([]byte{7}, KeySeedSize)
	a, _ := NewKeyFromSeed(SchemeSecp256k1, seed)
	b, _ := NewKeyFromSeed(SchemeSecp256k1, seed)
	if !bytes.Equal(a.PublicBytes(), b.PublicBytes()) {
		t.Fatal("the keygen must be deterministic")
	}
}

func TestUnknownSchemeKeyOps(t *testing.T) {
	key := &PrivateKey{Scheme: bogusScheme}

	if _, err := key.SignHash(make([]byte, HashSize)); err == nil {
		t.Fatal("signing under an unknown scheme must fail")
	}
	mustPanic(t, "PublicBytes", func() { key.PublicBytes() })
}

func TestVerifyHashSigRejects(t *testing.T) {
	hash := bytes.Repeat([]byte{0xaa}, HashSize)

	// size mismatches short-circuit
	if VerifyHashSig(SchemeSecp256k1, []byte{1}, hash, []byte{2}) {
		t.Fatal("wrong sizes must not verify")
	}
	// an unknown scheme (both sizes are 0, so nil/nil "fits")
	if VerifyHashSig(bogusScheme, nil, hash, nil) {
		t.Fatal("an unknown scheme must not verify")
	}

	// right sizes, garbage content, every scheme
	for _, scheme := range []SigScheme{SchemeSecp256k1, SchemeFNDSA512, SchemeMLDSA44, SchemeSLHDSA128} {
		pub := make([]byte, PubKeySize(scheme))
		sig := make([]byte, SigSize(scheme))
		if VerifyHashSig(scheme, pub, hash, sig) {
			t.Fatalf("scheme %#02x: zero-filled sig must not verify", byte(scheme))
		}
	}

	// a valid signature against the WRONG public key
	key := seededKey(t, SchemeSecp256k1, 0x10)
	other := seededKey(t, SchemeSecp256k1, 0x11)
	sig, err := key.SignHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyHashSig(SchemeSecp256k1, key.PublicBytes(), hash, sig) {
		t.Fatal("the right key must verify")
	}
	if VerifyHashSig(SchemeSecp256k1, other.PublicBytes(), hash, sig) {
		t.Fatal("a foreign key must not verify")
	}
}

func TestRecoverPubKey(t *testing.T) {
	hash := bytes.Repeat([]byte{0xbb}, HashSize)
	key := seededKey(t, SchemeSecp256k1, 0x12)
	sig, _ := key.SignHash(hash)

	if got := RecoverPubKey(SchemeSecp256k1, hash, sig); !bytes.Equal(got, key.PublicBytes()) {
		t.Fatal("recovery must yield the signer's key")
	}

	if RecoverPubKey(SchemeMLDSA44, hash, sig) != nil {
		t.Fatal("non-recovery schemes must return nil")
	}
	if RecoverPubKey(SchemeSecp256k1, hash, sig[:10]) != nil {
		t.Fatal("a short signature must return nil")
	}
	if RecoverPubKey(SchemeSecp256k1, hash, make([]byte, SigSize(SchemeSecp256k1))) != nil {
		t.Fatal("a garbage signature must return nil")
	}
}
