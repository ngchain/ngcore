package ngtypes

import (
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/utils"
)

// ErrAddressLenInvalid means the raw bytes are not exactly AddressSize long
var ErrAddressLenInvalid = errors.New("address length is invalid")

// addressVersion is a domain-separation byte for the address
// preimage, so any future layout change cannot collide with this one
const addressVersion = 0x01

// Address is the keccak-256 hash of the owner's public key — which
// may be the COMBINATION of several co-signers' keys (schnorr scalar
// addition), making shared wallets indistinguishable from plain ones.
// The key itself only appears on chain inside a spending tx's
// signature, keeping unspent funds shielded until spend time
type Address [AddressSize]byte

// AddressOfPubKey computes keccak256(version || pubkey)
func AddressOfPubKey(pubKey []byte) Address {
	addr := Address{}

	preimage := make([]byte, 0, 1+len(pubKey))
	preimage = append(preimage, addressVersion)
	preimage = append(preimage, pubKey...)

	copy(addr[:], utils.KeccakSum256(preimage))
	return addr
}

// NewAddress returns the address of a key
func NewAddress(key *PrivateKey) Address {
	return AddressOfPubKey(key.PublicBytes())
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
