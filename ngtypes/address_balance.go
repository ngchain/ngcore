package ngtypes

import "math/big"

type Balance struct {
	Address Address
	Amount  *big.Int
}
