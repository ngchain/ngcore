package ngtypes

import (
	"crypto/rand"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/slhdsa"
	"github.com/pkg/errors"
	"github.com/pornin/go-fn-dsa/fndsa"
	"golang.org/x/crypto/sha3"
)

// The chain accepts a small menu of post-quantum signature schemes,
// selectable per key. All of them derive from a 32-byte wallet seed
// and sign the 32-byte tx digest; they differ in the size/assumption
// trade-off:
//
//	FN-DSA-512   (default) — the compact one: 897 B key + 666 B sig,
//	             NTRU lattices, FIPS 206 draft
//	ML-DSA-44    — the finalized one: 1312 B key + 2420 B sig,
//	             module lattices, FIPS 204
//	SLH-DSA-128s — the assumption-minimal one: 32 B key + 7856 B sig,
//	             hash-based, FIPS 205
//
// A future aggregation layer can compress any mix of them: witness
// data is committed separately from the tx ids, so signatures can be
// replaced by aggregate proofs without touching history commitments.
type SigScheme byte

const (
	SchemeFNDSA512  SigScheme = 0x01
	SchemeMLDSA44   SigScheme = 0x02
	SchemeSLHDSA128 SigScheme = 0x03
)

// KeySeedSize is the wallet secret: a 32-byte seed the whole key pair
// regenerates from, whatever the scheme
const KeySeedSize = 32

var (
	ErrSchemeUnknown = errors.New("unknown signature scheme")
	ErrKeyInvalid    = errors.New("invalid private key")
)

const slhdsaParam = slhdsa.SHAKE_128s

// txDomain separates this chain's signatures from any other use of
// the same keys
var txDomain = []byte("ngcore")

// PubKeySize returns the encoded public key length of the scheme
// (0 for an unknown scheme)
func PubKeySize(scheme SigScheme) int {
	switch scheme {
	case SchemeFNDSA512:
		return fndsa.VerifyingKeySize(9)
	case SchemeMLDSA44:
		return mldsa44.PublicKeySize
	case SchemeSLHDSA128:
		return 32
	default:
		return 0
	}
}

// SigSize returns the signature length of the scheme (0 for an
// unknown scheme)
func SigSize(scheme SigScheme) int {
	switch scheme {
	case SchemeFNDSA512:
		return fndsa.SignatureSize(9)
	case SchemeMLDSA44:
		return mldsa44.SignatureSize
	case SchemeSLHDSA128:
		return 7856
	default:
		return 0
	}
}

// PrivateKey is a signing key under a chosen scheme. The wallet
// format (Serialize) is scheme byte + 32-byte seed
type PrivateKey struct {
	Scheme SigScheme

	seed [KeySeedSize]byte

	fndsaSK []byte
	fndsaPK []byte

	mldsaPub  *mldsa44.PublicKey
	mldsaPriv *mldsa44.PrivateKey

	slhPub  slhdsa.PublicKey
	slhPriv slhdsa.PrivateKey
}

// GenerateKey creates a fresh key under the default scheme (FN-DSA-512)
func GenerateKey() (*PrivateKey, error) {
	return GenerateSchemeKey(SchemeFNDSA512)
}

// GenerateSchemeKey creates a fresh key under the given scheme
func GenerateSchemeKey(scheme SigScheme) (*PrivateKey, error) {
	seed := make([]byte, KeySeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}

	return NewKeyFromSeed(scheme, seed)
}

// NewKeyFromSeed derives the key pair from a 32-byte secret seed: the
// keygen consumes a deterministic SHAKE256 stream, so the same seed
// always regenerates the same keys
func NewKeyFromSeed(scheme SigScheme, seed []byte) (*PrivateKey, error) {
	if len(seed) != KeySeedSize {
		return nil, errors.Wrapf(ErrKeyInvalid, "seed must be %d bytes, got %d", KeySeedSize, len(seed))
	}

	drbg := sha3.NewShake256()
	drbg.Write([]byte("ngcore-keygen"))
	drbg.Write([]byte{byte(scheme)})
	drbg.Write(seed)

	key := &PrivateKey{Scheme: scheme}
	copy(key.seed[:], seed)

	switch scheme {
	case SchemeFNDSA512:
		sk, pk, err := fndsa.KeyGen(9, drbg)
		if err != nil {
			return nil, err
		}
		key.fndsaSK, key.fndsaPK = sk, pk

	case SchemeMLDSA44:
		var mseed [mldsa44.SeedSize]byte
		drbg.Read(mseed[:])
		key.mldsaPub, key.mldsaPriv = mldsa44.NewKeyFromSeed(&mseed)

	case SchemeSLHDSA128:
		pub, priv, err := slhdsa.GenerateKey(drbg, slhdsaParam)
		if err != nil {
			return nil, err
		}
		key.slhPub, key.slhPriv = pub, priv

	default:
		return nil, errors.Wrapf(ErrSchemeUnknown, "scheme %#02x", byte(scheme))
	}

	return key, nil
}

// ParsePrivateKey reads the wallet format: scheme byte + 32-byte seed
func ParsePrivateKey(raw []byte) (*PrivateKey, error) {
	if len(raw) != 1+KeySeedSize {
		return nil, errors.Wrapf(ErrKeyInvalid, "serialized key must be %d bytes, got %d", 1+KeySeedSize, len(raw))
	}

	return NewKeyFromSeed(SigScheme(raw[0]), raw[1:])
}

// Serialize returns the wallet format: scheme byte + 32-byte seed
func (key *PrivateKey) Serialize() []byte {
	out := make([]byte, 1+KeySeedSize)
	out[0] = byte(key.Scheme)
	copy(out[1:], key.seed[:])

	return out
}

// PublicBytes returns the scheme-specific public key encoding
func (key *PrivateKey) PublicBytes() []byte {
	switch key.Scheme {
	case SchemeFNDSA512:
		out := make([]byte, len(key.fndsaPK))
		copy(out, key.fndsaPK)
		return out

	case SchemeMLDSA44:
		raw, err := key.mldsaPub.MarshalBinary()
		if err != nil {
			panic(err) // packing a valid key cannot fail
		}
		return raw

	case SchemeSLHDSA128:
		raw, err := key.slhPub.MarshalBinary()
		if err != nil {
			panic(err)
		}
		return raw

	default:
		panic(errors.Wrapf(ErrSchemeUnknown, "scheme %#02x", byte(key.Scheme)))
	}
}

// SignHash signs a 32-byte digest under the key's scheme
func (key *PrivateKey) SignHash(hash []byte) ([]byte, error) {
	switch key.Scheme {
	case SchemeFNDSA512:
		return fndsa.Sign(nil, key.fndsaSK, fndsa.DomainContext(txDomain), 0, hash)

	case SchemeMLDSA44:
		sig := make([]byte, mldsa44.SignatureSize)
		if err := mldsa44.SignTo(key.mldsaPriv, hash, txDomain, false, sig); err != nil {
			return nil, err
		}
		return sig, nil

	case SchemeSLHDSA128:
		return slhdsa.SignDeterministic(&key.slhPriv, slhdsa.NewMessage(hash), txDomain)

	default:
		return nil, errors.Wrapf(ErrSchemeUnknown, "scheme %#02x", byte(key.Scheme))
	}
}

// VerifyHashSig checks one signature over a 32-byte digest against a
// scheme-tagged public key
func VerifyHashSig(scheme SigScheme, pubKey, hash, sig []byte) bool {
	if len(pubKey) != PubKeySize(scheme) || len(sig) != SigSize(scheme) {
		return false
	}

	switch scheme {
	case SchemeFNDSA512:
		return fndsa.Verify(pubKey, fndsa.DomainContext(txDomain), 0, hash, sig)

	case SchemeMLDSA44:
		pub := new(mldsa44.PublicKey)
		if err := pub.UnmarshalBinary(pubKey); err != nil {
			return false
		}
		return mldsa44.Verify(pub, hash, txDomain, sig)

	case SchemeSLHDSA128:
		pub := slhdsa.PublicKey{ID: slhdsaParam}
		if err := pub.UnmarshalBinary(pubKey); err != nil {
			return false
		}
		return slhdsa.Verify(&pub, slhdsa.NewMessage(hash), sig, txDomain)

	default:
		return false
	}
}
