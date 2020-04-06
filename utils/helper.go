package utils

import (
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
