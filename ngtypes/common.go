package ngtypes

import "math/big"

// some message knowledge

// GetBig0 returns a new big 0.
func GetBig0() *big.Int {
	return big.NewInt(0)
}

// GetBig0Bytes returns a new big 0's bytes.
func GetBig0Bytes() []byte {
	return big.NewInt(0).Bytes()
}

// Big1 returns a big 1.
var Big1 = big.NewInt(1)

// Big2 returns a big 2.
var Big2 = big.NewInt(2)
