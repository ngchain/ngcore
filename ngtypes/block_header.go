package ngtypes

import (
	"github.com/ngchain/ngcore/ngtypes/ngproto"
	"google.golang.org/protobuf/proto"
)

func (x *Block) MarshalHeader() ([]byte, error) {
	header := proto.Clone(x.BlockHeader).(*ngproto.BlockHeader)

	return proto.Marshal(header)
}
