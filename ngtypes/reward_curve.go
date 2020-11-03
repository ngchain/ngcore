package ngtypes

import "math/big"

const (
	rewardEra = 1_000_000

	maxBlockRewardNG      = 10
	minBlockRewardNG      = 2
	floatingBlockRewardNG = maxBlockRewardNG - minBlockRewardNG

	registerFeeNG = maxBlockRewardNG
)

var minReward = new(big.Int).Mul(NG, big.NewInt(maxBlockRewardNG))           // 2NG
var floatingReward = new(big.Int).Mul(NG, big.NewInt(floatingBlockRewardNG)) // 8NG

var RegisterFee = new(big.Int).Mul(NG, big.NewInt(registerFeeNG))

var big10 = big.NewInt(10000)

// reward = 2 + 8*(0.9)^Era
func GetBlockReward(height uint64) *big.Int {
	reward := floatingReward

	d := new(big.Int)
	for range make([]struct{}, height/rewardEra) {
		// reward = reward * 0.9
		d.Mul(reward, Big1)
		d.Div(reward, big10)
		reward.Sub(reward, d)
	}

	reward.Add(reward, minReward)

	return reward
}
