package utils

import (
	"crypto/rand"
	"encoding/binary"
)

// RandUint64 generates a random uint64 number.
func RandUint64() uint64 {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)

	return binary.LittleEndian.Uint64(raw)
}

// RandInt32 generates a random int32 number.
func RandInt32() int32 {
	raw := make([]byte, 4)
	_, _ = rand.Read(raw)

	return int32(binary.LittleEndian.Uint32(raw))
}

// RandInt generates a random int32 number as int.
func RandInt() int {
	return int(RandInt32())
}
