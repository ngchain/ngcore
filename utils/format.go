package utils

import (
	"encoding/hex"
	"fmt"
)

// Bytes2Hex is a helper func to convert raw bytes to hex string
func Bytes2Hex(b []byte) string {
	s := ""
	for i := 0; i < len(b); i++ {
		s += fmt.Sprintf("%02X", b[i])
	}
	return s
}

// Hex2Bytes is a helper func to convert hex string to raw bytes
func Hex2Bytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
