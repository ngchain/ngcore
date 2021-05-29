package ngtypes

import (
	"github.com/ngchain/ngcore/ngtypes/ngproto"
	"google.golang.org/protobuf/proto"
)

type Account struct {
	Proto *ngproto.Account
}

// NewAccount receive parameters and return a new Account(class constructor.
func NewAccount(num AccountNum, ownerAddress, contract, context []byte) *Account {
	return &Account{
		&ngproto.Account{
			Num:      uint64(num),
			Owner:    ownerAddress,
			Contract: contract,
			Context:  context,
		},
	}
}

func NewAccountFromProto(protoAccount *ngproto.Account) *Account {
	return &Account{Proto: protoAccount}
}

// GetGenesisStyleAccount will return the genesis style account.
func GetGenesisStyleAccount(num AccountNum) *Account {
	return NewAccount(num, GenesisAddress, nil, nil)
}

func (x *Account) GetProto() *ngproto.Account {
	return x.Proto
}

func (x *Account) Marshal() ([]byte, error) {
	protoAccount := proto.Clone(x.GetProto()).(*ngproto.Account)

	return proto.Marshal(protoAccount)
}

func (x *Account) Equals(other *Account) (bool, error) {
	if !proto.Equal(x.Proto, other.Proto) {
		return false, nil
	}

	return true, nil
}
