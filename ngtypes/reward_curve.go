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
)

var (
	minReward      = new(big.Int).Mul(NG, big.NewInt(minBlockRewardNG))      // 2NG
	floatingReward = new(big.Int).Mul(NG, big.NewInt(floatingBlockRewardNG)) // 8NG
)

var big10 = big.NewInt(10)

var ErrRewardInvalid = errors.New("block reward is invalid")

// GetBlockReward returns the block reward in a specific height
// reward = 2 + 8*(0.9)^Era
func GetBlockReward(height uint64) *big.Int {
	reward := new(big.Int).Set(floatingReward)

	d := new(big.Int)
	era := height / rewardEra
	for i := uint64(0); i < era; i++ {
		// reward -= reward/10, i.e. reward *= 0.9 — pure integer math,
		// no floats anywhere near consensus. (The old code divided by a
		// mistyped 10000 and carried a dead Mul, decaying ~0.9999/era
		// against the documented curve.)
		d.Div(reward, big10)
		reward.Sub(reward, d)
	}

	reward.Add(reward, minReward)

	return reward
}

// UncleReward is what a nephew at nephewHeight pays the miner of an uncle
// at uncleHeight: the uncle's own block reward, decayed linearly to zero as
// the fork point recedes — full-ish reward one generation back, nothing at
// the UncleMaxDepth horizon. depth = nephewHeight - uncleHeight, in
// [1, UncleMaxDepth]. This compensates orphaned honest work (income
// fairness) on top of the GHOST work-weighting that already secures it.
func UncleReward(uncleHeight, nephewHeight uint64) *big.Int {
	if nephewHeight <= uncleHeight {
		return big.NewInt(0)
	}
	depth := nephewHeight - uncleHeight
	if depth > UncleMaxDepth {
		return big.NewInt(0)
	}

	// reward * (UncleMaxDepth + 1 - depth) / (UncleMaxDepth + 1)
	base := GetBlockReward(uncleHeight)
	num := big.NewInt(int64(UncleMaxDepth + 1 - depth))
	den := big.NewInt(int64(UncleMaxDepth + 1))

	out := new(big.Int).Mul(base, num)
	out.Quo(out, den)
	return out
}
