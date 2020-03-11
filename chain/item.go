package chain

import (
	"errors"
	"github.com/gogo/protobuf/proto"
)

var (
	ErrNoItemInHash         = errors.New("cannot find the item by hash")
	ErrNoItemHashInHeight   = errors.New("cannot find the item hash by height")
	ErrNoItemInHeight       = errors.New("cannot find the item by height")
	ErrItemHashInSameHeight = errors.New("already having the item hash in the same height")
	ErrNoHashInTag          = errors.New("no hash in tag")
	ErrNoHeightInTag        = errors.New("no height in tag")
)

const LatestHeightTag = "height"
const LatestHashTag = "hash"

// Item is an interface to block-like structures
type Item interface {
	proto.Message
	CalculateHash() ([]byte, error)
	GetHeight() uint64
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	GetPrevHash() []byte
}
