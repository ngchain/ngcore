package ngtypes

import (
	"github.com/mr-tron/base58"
	"github.com/ngchain/go-schnorr"
	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/utils"
)

// Address is the anonymous publickey for receiving coin
type Address [33]byte

// NewAddress will return a publickey address
func NewAddress(privKey *secp256k1.PrivateKey) Address {
	addr := [33]byte{}

	copy(addr[:], utils.PublicKey2Bytes(privKey.PubKey()))

	return addr
}

// NewAddressFromMultiKeys will return a publickey address
func NewAddressFromMultiKeys(privKeys ...*secp256k1.PrivateKey) (Address, error) {
	addr := [33]byte{}

	if len(privKeys) == 0 {
		panic("no private key entered")
	}

	pubKeys := make([]secp256k1.PublicKey, len(privKeys))
	pub := schnorr.CombinePublicKeys(pubKeys...)

	copy(addr[:], utils.PublicKey2Bytes(pub))
	return addr, nil
}

// NewAddressFromBS58 converts a base58 string into the Address
func NewAddressFromBS58(s string) (Address, error) {
	addr := [33]byte{}

	raw, err := base58.FastBase58Decoding(s)
	if err != nil {
		return Address{}, err
	}

	copy(addr[:], raw)
	return addr, nil
}

// PubKey gets the public key from address for validation
func (a Address) PubKey() *secp256k1.PublicKey {
	return utils.Bytes2PublicKey(a[:])
}

func (a Address) SetBytes(b []byte) Address {
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
func (a Address) UnmarshalJSON(b []byte) error {
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
