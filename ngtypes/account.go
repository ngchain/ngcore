package ngtypes

import "encoding/binary"

type AccountNum uint64

func (num AccountNum) Bytes() []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(num))
	return b
}

func NewNumFromBytes(b []byte) AccountNum {
	return AccountNum(binary.LittleEndian.Uint64(b))
}

// NewAccount receive parameters and return a new Account(class constructor.
func NewAccount(num AccountNum, ownerAddress, contract, context []byte) *Account {
	return &Account{
		Num:      uint64(num),
		Owner:    ownerAddress,
		Contract: contract,
		Context:  context,
	}
}

// GetGenesisStyleAccount will return the genesis style account.
func GetGenesisStyleAccount(num AccountNum) *Account {
	return &Account{
		Num:      uint64(num),
		Owner:    GenesisAddress,
		Contract: nil,
		Context:  nil,
	}
}
