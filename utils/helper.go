package utils

import (
	"bytes"
	"math/big"
)

// BigIntPlusPlus is a helper func to calculate i++ for big int i
func BigIntPlusPlus(bigInt *big.Int) *big.Int {
	return new(big.Int).Add(bigInt, big.NewInt(1))
}

// InBytesList is a helper func to check whether the sub bytes in the li list
func InBytesList(li [][]byte, sub []byte) (in bool) {
	for i := range li {
		if bytes.Equal(li[i], sub) {
			return true
		}
	}

	return false
}
