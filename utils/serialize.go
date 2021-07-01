package utils

import (
	"bytes"
	"encoding/binary"
)

// CombineBytes is a helper func to combine bytes without separator.
func CombineBytes(b ...[]byte) []byte {
	return bytes.Join(b, nil)
}

func InsertBytes(s []byte, k int, vs ...byte) []byte {
	if n := len(s) + len(vs); n <= cap(s) {
		s2 := s[:n]
		copy(s2[k+len(vs):], s[k:])
		copy(s2[k:], vs)
		return s2
	}
	s2 := make([]byte, len(s)+len(vs))
	copy(s2, s[:k])
	copy(s2[k:], vs)
	copy(s2[k+len(vs):], s[k:])
	return s2
}

func CutBytes(a []byte, i int, j int) []byte {
	copy(a[i:], a[j:])
	for k, n := len(a)-j+i, len(a); k < n; k++ {
		a[k] = 0 // or the zero value of T
	}

	return a[:len(a)-j+i]
}

// PackUint64LE converts int64 to bytes in LittleEndian.
func PackUint64LE(n uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n)

	return b
}

// PackUint64BE converts int64 to bytes in BigEndian.
func PackUint64BE(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)

	return b
}

// PackUint32LE converts int32 to bytes in LittleEndian.
func PackUint32LE(n uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, n)

	return b
}

// PackUint32BE converts int32 to bytes in BigEndian.
func PackUint32BE(n uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)

	return b
}

// PackUint16LE converts int16 to bytes in LittleEndian.
func PackUint16LE(n uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, n)
	return b
}

// PackUint16BE converts int16 to bytes in BigEndian.
func PackUint16BE(n uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, n)
	return b
}

// ReverseBytes converts bytes order between LittleEndian and BigEndian.
func ReverseBytes(b []byte) []byte {
	_b := make([]byte, len(b))
	copy(_b, b)

	for i, j := 0, len(_b)-1; i < j; i, j = i+1, j-1 {
		_b[i], _b[j] = _b[j], _b[i]
	}
	return _b
}
