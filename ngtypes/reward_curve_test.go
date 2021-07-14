package ngtypes

import "testing"

func TestGetBlockReward(t *testing.T) {
	t.Log(GetBlockReward(0))
	t.Log(GetBlockReward(100))
	t.Log(GetBlockReward(rewardEra))
	t.Log(GetBlockReward(2 * rewardEra))
	t.Log(GetBlockReward(4 * rewardEra))
}
