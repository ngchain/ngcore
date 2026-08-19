package utils

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/c0mm4nd/rlp"
)

// PackUint64LE converts int64 to bytes in LittleEndian.
func PackUint64LE(n uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n)

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

func HexRLPEncode(v any) string {
	rawBytes, err := rlp.EncodeToBytes(v)
	if err != nil {
		panic(err)
	}

	return hex.EncodeToString(rawBytes)
}

func HexRLPDecode(s string, v any) error {
	rawBytes, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	err = rlp.DecodeBytes(rawBytes, v)
	if err != nil {
		return err
	}

	return nil
}
