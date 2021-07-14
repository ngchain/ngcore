package ngtypes

import "encoding/binary"

// AccountNum is a uint64 number which used as the identifier of the Account
type AccountNum uint64

// Bytes convert the uint64 AccountNum into bytes in LE
func (num AccountNum) Bytes() []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(num))
	return b
}

// NewNumFromBytes recovers the AccountNum from the bytes in LE
func NewNumFromBytes(b []byte) AccountNum {
	return AccountNum(binary.LittleEndian.Uint64(b))
}
