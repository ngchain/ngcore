package ngtypes

import (
	"bytes"
	"github.com/c0mm4nd/rlp"
)

type Account struct {
	Num      uint64
	Owner    []byte
	Contract []byte
	Context  *AccountContext
}

// NewAccount receive parameters and return a new Account(class constructor.
func NewAccount(num AccountNum, ownerAddress, contract []byte, context *AccountContext) *Account {
	if context == nil {
		context = NewAccountContext()
	}

	return &Account{
		Num:      uint64(num),
		Owner:    ownerAddress,
		Contract: contract,
		Context:  context,
	}
}

// GetGenesisStyleAccount will return the genesis style account.
func GetGenesisStyleAccount(num AccountNum) *Account {
	return NewAccount(num, GenesisAddress, nil, nil)
}

func (x *Account) Marshal() ([]byte, error) {
	return rlp.EncodeToBytes(x)
}

func (x *Account) Equals(other *Account) (bool, error) {
	if !(x.Num == other.Num) {
		return false, nil
	}
	if !(bytes.Equal(x.Owner, other.Owner)) {
		return false, nil
	}
	if !(bytes.Equal(x.Contract, other.Contract)) {
		return false, nil
	}
	if eq, _ := x.Context.Equals(other.Context); !eq {
		return false, nil
	}

	return true, nil
}
