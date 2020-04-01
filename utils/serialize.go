package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
)

// 多个[]byte数组合并成一个[]byte
func CombineBytes(b ...[]byte) []byte {
	return bytes.Join(b, nil)
}

// Convert int64 to [] byte in LittleEndian form
func PackUint64LE(n uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n)
	return b
}

// Convert int64 to [] byte in BigEndian form
func PackUint64BE(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

// Convert int32 to [] byte in LittleEndian form
func PackUint32LE(n uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, n)
	return b
}

// Convert int32 to [] byte in BigEndian form
func PackUint32BE(n uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	return b
}

// Convert int16 to [] byte in LittleEndian form
func PackUint16LE(n uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, n)
	return b
}

// Convert int16 to [] byte in BigEndian form
func PackUint16BE(n uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, n)
	return b
}

// Encode base64 into string
func Base64EncodeToString(raw []byte) string {
	return base64.StdEncoding.EncodeToString(raw)
}

// Decode string to base64
func Base64DecodeString(b64 string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(b64)
}
