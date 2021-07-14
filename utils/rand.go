package utils

import (
	"crypto/rand"
	"encoding/binary"
)

// RandUint64 generates a random uint64 number(0 to 18446744073709551615).
func RandUint64() uint64 {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)

	return binary.LittleEndian.Uint64(raw)
}

// RandUint32 generates a random uint32 number(0 to 4294967295).
// Useful when getting a random port number.
func RandUint32() uint32 {
	raw := make([]byte, 4)
	_, _ = rand.Read(raw)

	return binary.LittleEndian.Uint32(raw)
}

// RandUint16 generates a random uint16 number(0 to 65535).
// Useful when getting a random port number.
func RandUint16() uint16 {
	raw := make([]byte, 2)
	_, _ = rand.Read(raw)

	return binary.LittleEndian.Uint16(raw)
}

// RandInt64 generates a random int64 number (-9223372036854775808 to 9223372036854775807).
func RandInt64() int32 {
	return int32(RandUint64())
}

// RandInt32 generates a random int32 number (-2147483648 to 2147483647).
func RandInt32() int32 {
	return int32(RandUint32())
}

// RandInt16 generates a random int16 number (-32768 to 32767).
func RandInt16() int16 {
	return int16(RandUint16())
}
