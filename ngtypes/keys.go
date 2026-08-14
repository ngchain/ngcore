package ngtypes

import (
	"crypto/rand"

	"github.com/pkg/errors"
	"github.com/pornin/go-fn-dsa/fndsa"
	"golang.org/x/crypto/sha3"
)

// The chain has exactly one native signature system: FN-DSA-512
// (Falcon, the upcoming FIPS 206), chosen for its compact envelopes —
// an 897-byte verifying key plus a 666-byte signature, 2.4x smaller
// than ML-DSA-44 while staying on the NIST post-quantum track.
// Verification is integer-only, so consensus is deterministic across
// platforms; the floating-point machinery is confined to SIGNING,
// which happens wallet-side and never inside consensus.
const (
	// fndsaLogN selects the degree: 2^9 = 512
	fndsaLogN = 9

	// KeySeedSize is the wallet secret: a 32-byte seed the whole key
	// pair regenerates from
	KeySeedSize = 32
)

var (
	// PublicKeySize is the byte length of an encoded verifying key (897)
	PublicKeySize = fndsa.VerifyingKeySize(fndsaLogN)
	// TxSignatureSize is the byte length of one signature (666)
	TxSignatureSize = fndsa.SignatureSize(fndsaLogN)
)

// txDomain separates this chain's signatures from any other use of the
// same keys
var txDomain = fndsa.DomainContext("ngcore")

var ErrKeyInvalid = errors.New("invalid private key")

// PrivateKey is a signing key. The wallet format (Serialize) is the
// bare 32-byte seed, so key files stay tiny even though the encoded
// keys are large
type PrivateKey struct {
	seed [KeySeedSize]byte
	sk   []byte // encoded signing key
	pk   []byte // encoded verifying key
}

// GenerateKey creates a fresh key
func GenerateKey() (*PrivateKey, error) {
	seed := make([]byte, KeySeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}

	return NewKeyFromSeed(seed)
}

// NewKeyFromSeed derives the key pair from a 32-byte secret seed: the
// keygen consumes a deterministic SHAKE256 stream, so the same seed
// always regenerates the same keys
func NewKeyFromSeed(seed []byte) (*PrivateKey, error) {
	if len(seed) != KeySeedSize {
		return nil, errors.Wrapf(ErrKeyInvalid, "seed must be %d bytes, got %d", KeySeedSize, len(seed))
	}

	drbg := sha3.NewShake256()
	drbg.Write([]byte("ngcore-fndsa-keygen"))
	drbg.Write(seed)

	sk, pk, err := fndsa.KeyGen(fndsaLogN, drbg)
	if err != nil {
		return nil, err
	}

	key := &PrivateKey{sk: sk, pk: pk}
	copy(key.seed[:], seed)

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

// PublicBytes returns the encoded verifying key (PublicKeySize bytes)
func (key *PrivateKey) PublicBytes() []byte {
	out := make([]byte, len(key.pk))
	copy(out, key.pk)

	return out
}

// SignHash signs a 32-byte digest
func (key *PrivateKey) SignHash(hash []byte) ([]byte, error) {
	// nil rng = the OS RNG; fn-dsa hedges it with the key and message,
	// so even a weak source cannot leak the key
	return fndsa.Sign(nil, key.sk, txDomain, 0, hash)
}

// VerifyHashSig checks one signature over a 32-byte digest
func VerifyHashSig(pubKey, hash, sig []byte) bool {
	if len(pubKey) != PublicKeySize || len(sig) != TxSignatureSize {
		return false
	}

	return fndsa.Verify(pubKey, txDomain, 0, hash, sig)
}
