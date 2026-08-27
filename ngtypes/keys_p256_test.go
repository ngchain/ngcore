package ngtypes

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/ngchain/ngcore/utils"
)

// TestVerifyP256 pins the contract-only P-256 primitive: it accepts a genuine
// ECDSA-P256 signature in both accepted public-key encodings, rejects tampering,
// and — critically — must NOT be reachable through the account-level
// VerifyHashSig, so P-256 can never sign a transaction.
func TestVerifyP256(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	digest := utils.Hash256([]byte("passkey assertion digest"))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest)
	if err != nil {
		t.Fatal(err)
	}

	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:64])

	// 65-byte uncompressed SEC1 (0x04 ‖ X ‖ Y)
	pub65 := make([]byte, 65)
	pub65[0] = 0x04
	priv.X.FillBytes(pub65[1:33])
	priv.Y.FillBytes(pub65[33:65])
	// 64-byte bare X‖Y (the WebAuthn COSE_Key form)
	pub64 := pub65[1:]

	if !VerifyContractSig(SchemeSecp256r1, pub65, digest, sig) {
		t.Fatal("valid signature rejected with 65-byte uncompressed key")
	}
	if !VerifyContractSig(SchemeSecp256r1, pub64, digest, sig) {
		t.Fatal("valid signature rejected with 64-byte X‖Y key")
	}

	// wrong digest must not verify
	if VerifyContractSig(SchemeSecp256r1, pub65, utils.Hash256([]byte("other")), sig) {
		t.Fatal("signature verified against the wrong digest")
	}

	// a tampered signature must not verify
	bad := append([]byte{}, sig...)
	bad[0] ^= 0xff
	if VerifyContractSig(SchemeSecp256r1, pub65, digest, bad) {
		t.Fatal("tampered signature verified")
	}

	// a key from a different secret must not verify
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPub := make([]byte, 65)
	otherPub[0] = 0x04
	other.X.FillBytes(otherPub[1:33])
	other.Y.FillBytes(otherPub[33:65])
	if VerifyContractSig(SchemeSecp256r1, otherPub, digest, sig) {
		t.Fatal("signature verified under the wrong public key")
	}

	// malformed inputs
	if VerifyContractSig(SchemeSecp256r1, pub65, digest, sig[:63]) {
		t.Fatal("short signature accepted")
	}
	if VerifyContractSig(SchemeSecp256r1, pub65[:33], digest, sig) {
		t.Fatal("compressed/short key accepted")
	}
	if VerifyContractSig(SchemeSecp256r1, pub65, digest[:31], sig) {
		t.Fatal("short digest accepted")
	}

	// THE invariant: P-256 is contract-only and must never be an account scheme.
	if VerifyHashSig(SchemeSecp256r1, pub65, digest, sig) {
		t.Fatal("P-256 leaked into the account-level VerifyHashSig")
	}
	if PubKeySize(SchemeSecp256r1) != 0 || SigSize(SchemeSecp256r1) != 0 {
		t.Fatal("P-256 must not register account-scheme key/sig sizes")
	}
}

// TestSchemePartition pins the classical/PQ tag partition: classical schemes
// (secp256k1, secp256r1) live in 0x00–0x0F and read as non-PQ; the lattice/hash
// schemes live in 0x10–0xFF and read as PQ; the boundary is SchemePQMin.
func TestSchemePartition(t *testing.T) {
	classical := []SigScheme{SchemeSecp256k1, SchemeSecp256r1}
	pq := []SigScheme{SchemeFNDSA512, SchemeMLDSA44, SchemeSLHDSA128}

	for _, s := range classical {
		if s >= SchemePQMin {
			t.Fatalf("classical scheme %#02x is in the PQ range (>= %#02x)", byte(s), byte(SchemePQMin))
		}
		if IsPostQuantum(s) {
			t.Fatalf("IsPostQuantum(%#02x) = true, want false (classical)", byte(s))
		}
	}
	for _, s := range pq {
		if s < SchemePQMin {
			t.Fatalf("PQ scheme %#02x is in the classical range (< %#02x)", byte(s), byte(SchemePQMin))
		}
		if !IsPostQuantum(s) {
			t.Fatalf("IsPostQuantum(%#02x) = false, want true (post-quantum)", byte(s))
		}
	}

	// the sentinel 0x00 (no signature) is classical, not PQ
	if IsPostQuantum(SigScheme(0x00)) {
		t.Fatal("the 0x00 no-signature sentinel must not read as post-quantum")
	}
}
