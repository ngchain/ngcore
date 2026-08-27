package ngtypes

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
)

// SchemeSecp256r1 is the NIST P-256 (secp256r1) ECDSA scheme. Unlike the four
// account schemes in keys.go, it is DELIBERATELY NOT a valid transaction-signing
// scheme: VerifyHashSig (the account/tx path) does not know it, so no base-layer
// account can ever be controlled by a P-256 key. It is exposed to CONTRACTS ONLY,
// through crypto.verify (VerifyContractSig), because verifying P-256 in pure wasm
// is prohibitively expensive.
//
// Its reason to exist is application-layer account abstraction: a ng:validate
// hook can verify a passkey / WebAuthn assertion, because the hardware
// authenticators people actually have (Apple Secure Enclave, Android, Windows
// Hello) sign with P-256 and nothing else. P-256 is NOT post-quantum, so keeping
// it contract-opt-in rather than a native account scheme preserves the chain's PQ
// guarantees: a passkey account is a convenience account that trades PQ safety for
// onboarding, and that trade is confined to whoever opts into it.
//
// Its tag sits in the CLASSICAL range (0x02, right after secp256k1) — P-256 is a
// classical curve — so IsPostQuantum reports false for it. The classical range
// placement is orthogonal to its contract-only status: VerifyHashSig still does
// not handle it, so it can never sign a transaction.
const SchemeSecp256r1 SigScheme = 0x02

// verifyP256 verifies a raw ECDSA-P256 signature over a 32-byte digest.
//
//   - pubKey: an uncompressed SEC1 point — 65 bytes (0x04 ‖ X32 ‖ Y32) — or the
//     bare 64-byte X‖Y a WebAuthn COSE_Key yields. Compressed points are not
//     accepted; the caller passes the uncompressed key.
//   - hash: the 32-byte message digest. For WebAuthn the app-layer contract
//     computes sha256(authenticatorData ‖ sha256(clientDataJSON)) and passes it
//     here; this primitive only checks the ECDSA relation over that digest.
//   - sig: raw r ‖ s, 64 bytes (two 32-byte big-endian scalars). ASN.1/DER is not
//     accepted; the caller strips the wrapper. Malleability (s vs n-s) is
//     irrelevant to a verify-only primitive — both verify — so no low-s rule is
//     imposed.
//
// Verification is fully deterministic (only signing draws randomness), so it is
// safe in consensus. It relies on crypto/ecdsa.Verify, which internally rejects
// off-curve and identity points, so no separate on-curve check is needed.
func verifyP256(pubKey, hash, sig []byte) bool {
	if len(hash) != 32 || len(sig) != 64 {
		return false
	}

	var x, y *big.Int
	switch len(pubKey) {
	case 65:
		if pubKey[0] != 0x04 {
			return false
		}
		x = new(big.Int).SetBytes(pubKey[1:33])
		y = new(big.Int).SetBytes(pubKey[33:65])
	case 64:
		x = new(big.Int).SetBytes(pubKey[:32])
		y = new(big.Int).SetBytes(pubKey[32:64])
	default:
		return false
	}

	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])
	return ecdsa.Verify(pub, hash, r, s)
}

// VerifyContractSig is the signature-verification surface exposed to CONTRACTS
// via crypto.verify. It is a strict superset of the account-level VerifyHashSig:
// the four native schemes behave identically, plus SchemeSecp256r1 (P-256), which
// is contract-only. Keeping this distinct from VerifyHashSig is the whole point —
// contracts can verify P-256 without P-256 ever becoming a valid
// transaction-signing scheme.
func VerifyContractSig(scheme SigScheme, pubKey, hash, sig []byte) bool {
	if scheme == SchemeSecp256r1 {
		return verifyP256(pubKey, hash, sig)
	}
	return VerifyHashSig(scheme, pubKey, hash, sig)
}
