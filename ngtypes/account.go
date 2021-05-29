package ngtypes

import (
	"fmt"

	"github.com/ngchain/ngcore/ngtypes/ngproto"
	"google.golang.org/protobuf/proto"
)

type Account struct {
	*ngproto.Account
}

func (x *Account) GetProto() *ngproto.Account {
	return x.Account
}

func (*Account) ProtoMessage() error {
	return fmt.Errorf("not a proto")
}

func (x *Account) Marshal() ([]byte, error) {
	protoAccount := proto.Clone(x.GetProto()).(*ngproto.Account)

	return proto.Marshal(protoAccount)
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

// GetGenesisStyleAccount will return the genesis style account.
func GetGenesisStyleAccount(num AccountNum) *Account {
	return NewAccount(num, GenesisAddress, nil, nil)
}
