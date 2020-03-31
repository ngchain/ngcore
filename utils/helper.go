package utils

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
)

func RandUint64() uint64 {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	return binary.LittleEndian.Uint64(raw)
}

func BigIntPlusPlus(bigInt *big.Int) *big.Int {
	return new(big.Int).Add(bigInt, big.NewInt(1))
}
