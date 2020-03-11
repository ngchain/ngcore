package utils

import "math/big"

func BigIntPlusPlus(bigInt *big.Int) *big.Int {
	return new(big.Int).Add(bigInt, big.NewInt(1))
}
