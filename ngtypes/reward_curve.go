package ngtypes

import (
	"math/big"

	"github.com/pkg/errors"
)

const (
	rewardEra = 1_000_000

	maxBlockRewardNG      = 10
	minBlockRewardNG      = 2
	floatingBlockRewardNG = maxBlockRewardNG - minBlockRewardNG

	registerFeeNG = maxBlockRewardNG
)

var (
	minReward      = new(big.Int).Mul(NG, big.NewInt(minBlockRewardNG))      // 2NG
	floatingReward = new(big.Int).Mul(NG, big.NewInt(floatingBlockRewardNG)) // 8NG
)

// RegisterFee is the fee for registering a new account
var RegisterFee = new(big.Int).Mul(NG, big.NewInt(registerFeeNG))

var (
	big1  = big.NewInt(1)
	big10 = big.NewInt(10000)
)

var ErrRewardInvalid = errors.New("block reward is invalid")

// GetBlockReward returns the block reward in a specific height
// reward = 2 + 8*(0.9)^Era
func GetBlockReward(height uint64) *big.Int {
	reward := new(big.Int).Set(floatingReward)

	d := new(big.Int)
	era := height / rewardEra
	for i := uint64(0); i < era; i++ {
		// reward = reward * 0.9
		d.Mul(reward, big1)
		d.Div(reward, big10)
		reward.Sub(reward, d)
	}

	reward.Add(reward, minReward)

	return reward
}
