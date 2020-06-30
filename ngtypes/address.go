package ngtypes

import (
	"github.com/mr-tron/base58"
	"github.com/ngchain/ngcore/utils"
	"github.com/ngchain/secp256k1"
)

// Address is the anonymous address for receiving coin, 2+33=35 length
type Address []byte

// NewAddress will return a 2+33=35 bytes length address
func NewAddress(privKey *secp256k1.PrivateKey) Address {
	checkSum := utils.Sha3Sum256(privKey.Serialize())[0:2]

	return append(checkSum, utils.PublicKey2Bytes(*privKey.PubKey())...)
}

func NewAddressFromBS58(s string) (Address, error) {
	addr, err := base58.FastBase58Decoding(s)
	if err != nil {
		return nil, err
	}

	return addr, nil
}

// PubKey gets the public key from address for validition
func (a Address) PubKey() secp256k1.PublicKey {
	return utils.Bytes2PublicKey(a[2:])
}

func (a Address) BS58() string {
	return base58.FastBase58Encoding(a)
}

func (a Address) String() string {
	return a.BS58()
}

func (a Address) MarshalJSON() ([]byte, error) {
	raw := base58.FastBase58Encoding(a)

	return utils.JSON.Marshal(raw)
}

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
