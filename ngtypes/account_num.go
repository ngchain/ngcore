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
