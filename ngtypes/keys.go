package ngtypes

import (
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/pkg/errors"
)

// The chain has exactly one native signature system: secp256k1 BIP-340
// schnorr. Its scalar homomorphism is what powers the many-to-many
// txs: any number of co-signers combine into ONE key whose address
// receives and spends with a single 64-byte signature
const (
	// KeySeedSize is the wallet secret: the 32-byte scalar
	KeySeedSize = 32
	// PublicKeySize is the byte length of a compressed public key
	PublicKeySize = 33
	// TxSignatureSize is the byte length of one bip-340 signature
	TxSignatureSize = 64
)

var ErrKeyInvalid = errors.New("invalid private key")

// PrivateKey is a signing key (possibly the combination of several
// co-signers' keys). The wallet format (Serialize) is the bare
// 32-byte scalar
type PrivateKey struct {
	secp *btcec.PrivateKey
}

// GenerateKey creates a fresh key
func GenerateKey() (*PrivateKey, error) {
	secp, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	return &PrivateKey{secp: secp}, nil
}

// NewKeyFromSeed derives the key from the 32-byte secret scalar
func NewKeyFromSeed(seed []byte) (*PrivateKey, error) {
	if len(seed) != KeySeedSize {
		return nil, errors.Wrapf(ErrKeyInvalid, "seed must be %d bytes, got %d", KeySeedSize, len(seed))
	}

	secp, _ := btcec.PrivKeyFromBytes(seed)
	if secp.Key.IsZero() {
		return nil, errors.Wrap(ErrKeyInvalid, "the scalar is zero")
	}

	return &PrivateKey{secp: secp}, nil
}

// ParsePrivateKey reads the wallet format: the bare 32-byte scalar
func ParsePrivateKey(raw []byte) (*PrivateKey, error) {
	return NewKeyFromSeed(raw)
}

// Serialize returns the wallet format: the bare 32-byte scalar
func (key *PrivateKey) Serialize() []byte {
	return key.secp.Serialize()
}

// PublicBytes returns the compressed public key (PublicKeySize bytes)
func (key *PrivateKey) PublicBytes() []byte {
	return key.secp.PubKey().SerializeCompressed()
}

// SignHash signs a 32-byte digest with a bip-340 schnorr signature
func (key *PrivateKey) SignHash(hash []byte) ([]byte, error) {
	sig, err := schnorr.Sign(key.secp, hash)
	if err != nil {
		return nil, err
	}

	return sig.Serialize(), nil
}

// VerifyHashSig checks one signature over a 32-byte digest against a
// compressed public key (bip-340: the x-only form of the key verifies)
func VerifyHashSig(pubKey, hash, sig []byte) bool {
	pub, err := btcec.ParsePubKey(pubKey)
	if err != nil {
		return false
	}

	parsed, err := schnorr.ParseSignature(sig)
	if err != nil {
		return false
	}

	return parsed.Verify(hash, pub)
}

// CombinePrivateKeys folds any number of co-signers' keys into the
// single key by scalar addition: its public key equals the sum of the
// members' public keys, so the combined address spends with one plain
// signature — the many-to-many primitive
func CombinePrivateKeys(privKeys ...*PrivateKey) (*PrivateKey, error) {
	if len(privKeys) == 0 {
		return nil, errors.Wrap(ErrKeyInvalid, "no private key entered")
	}

	d := new(big.Int)
	for i := range privKeys {
		d.Add(d, new(big.Int).SetBytes(privKeys[i].Serialize()))
	}
	d.Mod(d, btcec.S256().N)
	if d.Sign() == 0 {
		return nil, errors.Wrap(ErrKeyInvalid, "the combined scalar is zero")
	}

	return NewKeyFromSeed(d.FillBytes(make([]byte, KeySeedSize)))
}

// NewAddressFromMultiKeys returns the address of the combined key: the
// all-must-sign shared wallet of the given co-signers
func NewAddressFromMultiKeys(privKeys ...*PrivateKey) (Address, error) {
	key, err := CombinePrivateKeys(privKeys...)
	if err != nil {
		return Address{}, err
	}

	return NewAddress(key), nil
}
