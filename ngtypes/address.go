package ngtypes

import (
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/utils"
)

// ErrAddressLenInvalid means the raw bytes are not exactly AddressSize long
var ErrAddressLenInvalid = errors.New("address length is invalid")

// Address is the anonymous publickey for receiving coin
type Address [AddressSize]byte

// NewAddress will return a publickey address
func NewAddress(privKey *btcec.PrivateKey) Address {
	addr := Address{}

	copy(addr[:], utils.PublicKey2Bytes(privKey.PubKey()))

	return addr
}

// NewAddressFromMultiKeys will return a publickey address: the shards
// of one owner combine into a single key by scalar addition, so the
// address is the plain pubkey of the combined secret and signatures
// stay standard BIP-340
func NewAddressFromMultiKeys(privKeys ...*btcec.PrivateKey) (Address, error) {
	key, err := CombinePrivateKeys(privKeys...)
	if err != nil {
		return Address{}, err
	}

	return NewAddress(key), nil
}

// ErrKeysInvalid means the private keys cannot form a usable combined key
var ErrKeysInvalid = errors.New("invalid private keys")

// CombinePrivateKeys folds one owner's key shards into the single
// secret whose public key is the sum of the shard public keys
func CombinePrivateKeys(privKeys ...*btcec.PrivateKey) (*btcec.PrivateKey, error) {
	if len(privKeys) == 0 {
		return nil, errors.Wrap(ErrKeysInvalid, "no private key entered")
	}

	d := new(big.Int)
	for i := range privKeys {
		d.Add(d, new(big.Int).SetBytes(privKeys[i].Serialize()))
	}
	d.Mod(d, btcec.S256().N)
	if d.Sign() == 0 {
		return nil, errors.Wrap(ErrKeysInvalid, "the combined secret is zero")
	}

	key, _ := btcec.PrivKeyFromBytes(d.FillBytes(make([]byte, 32)))
	return key, nil
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

// NewAddressFromLegacyBS58 reads the pre-2022 35-byte address format —
// a 2-byte private-key checksum followed by the 33-byte compressed
// public key — and returns the canonical pubkey address. Only the
// genesis sheet still speaks this format
func NewAddressFromLegacyBS58(s string) (Address, error) {
	addr := Address{}

	raw, err := base58.FastBase58Decoding(s)
	if err != nil {
		return addr, err
	}
	if len(raw) != AddressSize+2 {
		return addr, errors.Wrapf(ErrAddressLenInvalid, "legacy %q decodes to %d bytes", s, len(raw))
	}

	copy(addr[:], raw[2:])
	return addr, nil
}

// PubKey gets the public key from address for validation; nil when the
// address bytes are not a valid curve point
func (a Address) PubKey() *btcec.PublicKey {
	return utils.Bytes2PublicKey(a[:])
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

	copy(a[:], addr)
	return nil
}
