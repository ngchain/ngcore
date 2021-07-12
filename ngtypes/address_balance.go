package ngtypes

import "math/big"

// Balance is a unit in Sheet.Balances, which represents the remaining
// coin amount of the address
type Balance struct {
	Address Address
	Amount  *big.Int
}
