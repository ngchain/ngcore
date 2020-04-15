package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"math/big"
)

// RandUint64 generates a random uint64 number
func RandUint64() uint64 {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	return binary.LittleEndian.Uint64(raw)
}

// BigIntPlusPlus is a helper func to calculate i++ for big int i
func BigIntPlusPlus(bigInt *big.Int) *big.Int {
	return new(big.Int).Add(bigInt, big.NewInt(1))
}

func InBytesList(li [][]byte, sub []byte) (in bool) {
	for i := range li {
		if bytes.Equal(li[i], sub) {
			return true
		}
	}

	return false
}
