package ngtypes

import (
	"encoding/binary"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/utils"
)

// ErrAddressLenInvalid means the raw bytes are not exactly AddressSize long
var ErrAddressLenInvalid = errors.New("address length is invalid")

// ErrKeysetInvalid means a keyset descriptor is malformed
var ErrKeysetInvalid = errors.New("invalid keyset")

// MaxKeysetKeys bounds how many member keys one address may commit to
const MaxKeysetKeys = 16

// addressVersion tags the descriptor preimage, so future address
// layouts can never collide with the current one
const addressVersion = 0x01

// Address is the keccak-256 hash of a keyset descriptor: it commits to
// a signing threshold and the member public keys without revealing
// them. Public keys only appear on chain inside a spending tx's
// signature, which keeps unspent funds shielded and post-quantum keys
// (1.3 KB public keys) usable as compact addresses
type Address [AddressSize]byte

// KeysetAddress computes the address committing to threshold-of-keys:
//
//	keccak256(version || threshold || N × (scheme || len(pub) BE16 || pub))
func KeysetAddress(threshold int, schemes []SigScheme, pubKeys [][]byte) (Address, error) {
	addr := Address{}

	if len(schemes) != len(pubKeys) {
		return addr, errors.Wrap(ErrKeysetInvalid, "schemes and keys misaligned")
	}
	if len(pubKeys) == 0 || len(pubKeys) > MaxKeysetKeys {
		return addr, errors.Wrapf(ErrKeysetInvalid, "%d member keys", len(pubKeys))
	}
	if threshold < 1 || threshold > len(pubKeys) {
		return addr, errors.Wrapf(ErrKeysetInvalid, "threshold %d of %d keys", threshold, len(pubKeys))
	}

	preimage := []byte{addressVersion, byte(threshold)}
	for i := range pubKeys {
		if len(pubKeys[i]) == 0 || len(pubKeys[i]) > 1<<16-1 {
			return addr, errors.Wrapf(ErrKeysetInvalid, "member %d key size %d", i, len(pubKeys[i]))
		}
		preimage = append(preimage, byte(schemes[i]))
		preimage = binary.BigEndian.AppendUint16(preimage, uint16(len(pubKeys[i])))
		preimage = append(preimage, pubKeys[i]...)
	}

	copy(addr[:], utils.KeccakSum256(preimage))
	return addr, nil
}

// NewAddress returns the 1-of-1 address of a single key
func NewAddress(key *PrivateKey) Address {
	addr, err := KeysetAddress(1, []SigScheme{key.Scheme}, [][]byte{key.PublicBytes()})
	if err != nil {
		panic(err) // a single well-formed key cannot fail
	}

	return addr
}

// NewMultisigAddress commits to native threshold-of-N multisig over the
// given keys; the schemes may mix (e.g. one secp + one ML-DSA shard)
func NewMultisigAddress(threshold int, privKeys ...*PrivateKey) (Address, error) {
	schemes := make([]SigScheme, len(privKeys))
	pubKeys := make([][]byte, len(privKeys))
	for i := range privKeys {
		schemes[i] = privKeys[i].Scheme
		pubKeys[i] = privKeys[i].PublicBytes()
	}

	return KeysetAddress(threshold, schemes, pubKeys)
}

// NewAddressFromMultiKeys is the all-must-sign form: N-of-N multisig
func NewAddressFromMultiKeys(privKeys ...*PrivateKey) (Address, error) {
	return NewMultisigAddress(len(privKeys), privKeys...)
}

// mustAddressFromBS58 is NewAddressFromBS58 for hardcoded constants:
// it panics at init instead of silently yielding a wrong address
func mustAddressFromBS58(s string) Address {
	addr, err := NewAddressFromBS58(s)
	if err != nil {
		panic("bad address constant " + s + ": " + err.Error())
	}

	return addr
}

// NewAddressFromBS58 converts a base58 string into the Address
func NewAddressFromBS58(s string) (Address, error) {
	addr := Address{}

	raw, err := base58.FastBase58Decoding(s)
	if err != nil {
		return addr, err
	}
	if len(raw) != AddressSize {
		return addr, errors.Wrapf(ErrAddressLenInvalid, "%q decodes to %d bytes", s, len(raw))
	}

	copy(addr[:], raw)
	return addr, nil
}

// SetBytes rebuilds the Address from its raw bytes; the length is an
// internal invariant, so a mismatch is a programming error
func (a Address) SetBytes(b []byte) Address {
	if len(b) != AddressSize {
		panic(errors.Wrapf(ErrAddressLenInvalid, "SetBytes with %d bytes", len(b)))
	}

	copy(a[:], b)

	return a
}

func (a Address) Bytes() []byte {
	return a[:]
}

// BS58 generates the base58 string representing the Address
func (a Address) BS58() string {
	return base58.FastBase58Encoding(a[:])
}

func (a Address) String() string {
	return a.BS58()
}

func (a Address) Equals(other Address) bool {
	return a == other
}

// MarshalJSON makes the base58 string as the Address' json value
func (a Address) MarshalJSON() ([]byte, error) {
	raw := base58.FastBase58Encoding(a[:])

	return utils.JSON.Marshal(raw)
}

// UnmarshalJSON recovers the Address from the base58 string json value
func (a *Address) UnmarshalJSON(b []byte) error {
	var bs58Addr string
	err := utils.JSON.Unmarshal(b, &bs58Addr)
	if err != nil {
		return err
	}

	addr, err := base58.FastBase58Decoding(bs58Addr)
	if err != nil {
		return err
	}
	if len(addr) != AddressSize {
		return errors.Wrapf(ErrAddressLenInvalid, "%q decodes to %d bytes", bs58Addr, len(addr))
	}

	copy(a[:], addr)
	return nil
}
