package ngtypes

import (
	"fmt"

	"github.com/ngchain/ngcore/ngtypes/ngproto"
	"google.golang.org/protobuf/proto"
)

type Sheet struct {
	*ngproto.Sheet
}

// NewSheet gets the rows from db and return the sheet for transport/saving.
func NewSheet(network ngproto.NetworkType, height uint64, blockHash []byte, accounts map[uint64]*ngproto.Account, anonymous map[string][]byte) *Sheet {
	return &Sheet{
		&ngproto.Sheet{
			Network:   network,
			Height:    height,
			BlockHash: blockHash,
			Anonymous: anonymous,
			Accounts:  accounts,
		},
	}
}

func NewSheetFromProto(protoSheet *ngproto.Sheet) *Sheet {
	return &Sheet{
		protoSheet,
	}
}

func (x *Sheet) GetProto() *ngproto.Sheet {
	return x.Sheet
}

func (*Sheet) ProtoMessage() error {
	return fmt.Errorf("not a proto")
}

func (x *Sheet) Marshal() ([]byte, error) {
	protoSheet := proto.Clone(x.GetProto())

	return proto.Marshal(protoSheet)
}
