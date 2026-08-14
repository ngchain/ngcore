package ngtypes

import (
	"crypto/rand"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/pkg/errors"
)

// SigScheme identifies the signature scheme of one keyset member. Only
// post-quantum schemes exist; the byte keeps the descriptor layout
// open for future additions (e.g. a hash-based fallback scheme)
type SigScheme byte

const (
	// SchemeMLDSA44 is ML-DSA-44 (FIPS 204, module-lattice signatures)
	SchemeMLDSA44 SigScheme = 0x02
)

// KeySeedSize is the byte length of every serialized secret: an
// ML-DSA keygen seed is 32 bytes
const KeySeedSize = 32

var (
	ErrSchemeUnknown = errors.New("unknown signature scheme")
	ErrKeyInvalid    = errors.New("invalid private key")
)

// PrivateKey is one keyset member's secret under a chosen scheme. The
// wallet format (Serialize) is always scheme byte + 32-byte seed, so
// key files stay tiny even for post-quantum keys
type PrivateKey struct {
	Scheme SigScheme

	mldsaSeed [mldsa44.SeedSize]byte
	mldsaPub  *mldsa44.PublicKey
	mldsaPriv *mldsa44.PrivateKey
}

// GenerateKey creates a fresh key under the default scheme
func GenerateKey() (*PrivateKey, error) {
	seed := make([]byte, KeySeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}

	return NewKeyFromSeed(SchemeMLDSA44, seed)
}

// NewKeyFromSeed derives the key of the scheme from a 32-byte secret seed
func NewKeyFromSeed(scheme SigScheme, seed []byte) (*PrivateKey, error) {
	if len(seed) != KeySeedSize {
		return nil, errors.Wrapf(ErrKeyInvalid, "seed must be %d bytes, got %d", KeySeedSize, len(seed))
	}

	key := &PrivateKey{Scheme: scheme}
	switch scheme {
	case SchemeMLDSA44:
		copy(key.mldsaSeed[:], seed)
		key.mldsaPub, key.mldsaPriv = mldsa44.NewKeyFromSeed(&key.mldsaSeed)
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
	copy(out[1:], key.mldsaSeed[:])

	return out
}

// PublicBytes returns the scheme-specific public key encoding:
// 1312 bytes for ML-DSA-44
func (key *PrivateKey) PublicBytes() []byte {
	switch key.Scheme {
	case SchemeMLDSA44:
		raw, err := key.mldsaPub.MarshalBinary()
		if err != nil {
			panic(err) // packing a valid key cannot fail
		}
		return raw
	default:
		panic(errors.Wrapf(ErrSchemeUnknown, "scheme %#02x", byte(key.Scheme)))
	}
}

// SignHash signs a 32-byte digest under the key's scheme
func (key *PrivateKey) SignHash(hash []byte) ([]byte, error) {
	switch key.Scheme {
	case SchemeMLDSA44:
		sig := make([]byte, mldsa44.SignatureSize)
		// deterministic signing: re-signing the same tx yields the
		// identical signature (and so the identical tx hash)
		if err := mldsa44.SignTo(key.mldsaPriv, hash, nil, false, sig); err != nil {
			return nil, err
		}
		return sig, nil
	default:
		return nil, errors.Wrapf(ErrSchemeUnknown, "scheme %#02x", byte(key.Scheme))
	}
}

// VerifyHashSig checks one signature over a 32-byte digest against a
// scheme-tagged public key
func VerifyHashSig(scheme SigScheme, pubKey, hash, sig []byte) bool {
	switch scheme {
	case SchemeMLDSA44:
		if len(pubKey) != mldsa44.PublicKeySize {
			return false
		}
		pub := new(mldsa44.PublicKey)
		if err := pub.UnmarshalBinary(pubKey); err != nil {
			return false
		}
		return mldsa44.Verify(pub, hash, nil, sig)
	default:
		return false
	}
}
