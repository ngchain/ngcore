package ngtypes

import (
	"crypto/rand"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/pkg/errors"
)

// The chain has exactly one native signature system: ML-DSA-44
// (FIPS 204, module-lattice signatures, post-quantum)
const (
	// KeySeedSize is the wallet secret: the 32-byte ML-DSA keygen seed
	KeySeedSize = 32
	// PublicKeySize is the byte length of a member public key
	PublicKeySize = mldsa44.PublicKeySize
	// TxSignatureSize is the byte length of one member signature
	TxSignatureSize = mldsa44.SignatureSize
)

var ErrKeyInvalid = errors.New("invalid private key")

// PrivateKey is one keyset member's secret. The wallet format
// (Serialize) is the bare 32-byte keygen seed, so key files stay tiny
// even though the post-quantum keys themselves are large
type PrivateKey struct {
	seed [mldsa44.SeedSize]byte
	pub  *mldsa44.PublicKey
	priv *mldsa44.PrivateKey
}

// GenerateKey creates a fresh key
func GenerateKey() (*PrivateKey, error) {
	seed := make([]byte, KeySeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}

	return NewKeyFromSeed(seed)
}

// NewKeyFromSeed derives the key from a 32-byte secret seed
func NewKeyFromSeed(seed []byte) (*PrivateKey, error) {
	if len(seed) != KeySeedSize {
		return nil, errors.Wrapf(ErrKeyInvalid, "seed must be %d bytes, got %d", KeySeedSize, len(seed))
	}

	key := &PrivateKey{}
	copy(key.seed[:], seed)
	key.pub, key.priv = mldsa44.NewKeyFromSeed(&key.seed)

	return key, nil
}

// ParsePrivateKey reads the wallet format: the bare 32-byte seed
func ParsePrivateKey(raw []byte) (*PrivateKey, error) {
	return NewKeyFromSeed(raw)
}

// Serialize returns the wallet format: the bare 32-byte seed
func (key *PrivateKey) Serialize() []byte {
	out := make([]byte, KeySeedSize)
	copy(out, key.seed[:])

	return out
}

// PublicBytes returns the packed public key (PublicKeySize bytes)
func (key *PrivateKey) PublicBytes() []byte {
	raw, err := key.pub.MarshalBinary()
	if err != nil {
		panic(err) // packing a valid key cannot fail
	}

	return raw
}

// SignHash signs a 32-byte digest
func (key *PrivateKey) SignHash(hash []byte) ([]byte, error) {
	sig := make([]byte, mldsa44.SignatureSize)
	// deterministic signing: re-signing the same tx yields the
	// identical signature (and so the identical tx hash)
	if err := mldsa44.SignTo(key.priv, hash, nil, false, sig); err != nil {
		return nil, err
	}

	return sig, nil
}

// VerifyHashSig checks one member signature over a 32-byte digest
func VerifyHashSig(pubKey, hash, sig []byte) bool {
	if len(pubKey) != mldsa44.PublicKeySize {
		return false
	}

	pub := new(mldsa44.PublicKey)
	if err := pub.UnmarshalBinary(pubKey); err != nil {
		return false
	}

	return mldsa44.Verify(pub, hash, nil, sig)
}
