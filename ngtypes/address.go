package ngtypes

import (
	"github.com/mr-tron/base58"
	"github.com/ngchain/go-schnorr"
	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/utils"
)

// Address is the anonymous address for receiving coin, 2+33=35 length
type Address []byte

// NewAddress will return a 2+33=35 bytes length address
func NewAddress(privKey *secp256k1.PrivateKey) Address {
	checkSum := utils.Sha3Sum256(privKey.Serialize())[0:2]

	return append(checkSum, utils.PublicKey2Bytes(*privKey.PubKey())...)
}

// NewAddressFromMultiKeys will return a 2+33=35 bytes length address
func NewAddressFromMultiKeys(privKeys ...*secp256k1.PrivateKey) (Address, error) {
	if len(privKeys) == 0 {
		panic("no private key entered")
	}

	pubKeys := make([]secp256k1.PublicKey, len(privKeys))
	allKeyBytes := make([]byte, 0, len(privKeys)*PrivSize)

	for i := 0; i < len(privKeys); i++ {
		pubKeys[i] = *privKeys[i].PubKey()
		allKeyBytes = append(allKeyBytes, privKeys[i].Serialize()...)
	}

	checkSum := utils.Sha3Sum256(allKeyBytes)[0:2]

	pub := schnorr.CombinePublicKeys(pubKeys...)

	return append(checkSum, utils.PublicKey2Bytes(*pub)...), nil
}

// NewAddressFromBS58 converts a base58 string into the Address
func NewAddressFromBS58(s string) (Address, error) {
	addr, err := base58.FastBase58Decoding(s)
	if err != nil {
		return nil, err
	}

	return addr, nil
}

// PubKey gets the public key from address for validation
func (a Address) PubKey() secp256k1.PublicKey {
	return utils.Bytes2PublicKey(a[2:])
}

// BS58 generates the base58 string representing the Address
func (a Address) BS58() string {
	return base58.FastBase58Encoding(a)
}

func (a Address) String() string {
	return a.BS58()
}

// MarshalJSON makes the base58 string as the Address' json value
func (a Address) MarshalJSON() ([]byte, error) {
	raw := base58.FastBase58Encoding(a)

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

	*a = addr
	return nil
}
