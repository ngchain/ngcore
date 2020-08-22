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

// GetBig1 returns a new big 1.
func GetBig1() *big.Int {
	return big.NewInt(1)
}

// GetBig2 returns a new big 2.
func GetBig2() *big.Int {
	return big.NewInt(2)
}
