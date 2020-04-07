package ngchain

import (
	"github.com/gogo/protobuf/proto"
)

// Item is an interface to block-like structures
type Item interface {
	proto.Message
	CalculateHash() ([]byte, error)
	GetHeight() uint64
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	GetPrevHash() []byte
}
