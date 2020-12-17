package ngtypes

import (
	"math/big"
	"time"
)

var big2 = big.NewInt(2)

// GetNextDiff is a helper to get next pow block Diff field.
func GetNextDiff(blockHeight uint64, blockTime int64, tailBlock *Block) *big.Int {
	diff := new(big.Int).SetBytes(tailBlock.GetDifficulty())
	if !tailBlock.IsTail() {
		return diff
	}

	elapsed := tailBlock.Timestamp - GetGenesisTimestamp(tailBlock.Network)
	diffTime := elapsed - int64(tailBlock.GetHeight())*int64(TargetTime/time.Second)
	delta := new(big.Int)
	if diffTime < int64(TargetTime/time.Second)*(-2) {
		delta.Div(diff, big.NewInt(10))
		diff.Add(diff, delta)
	}

	if diffTime > int64(TargetTime/time.Second)*(+2) {
		delta.Div(diff, big.NewInt(10))
		diff.Sub(diff, delta)
	}

	period := (tailBlock.Height + 1) / 1000
	// TODO: delete me
	if (tailBlock.Height+1)%1000 == 0 && period > 10 && period < 26 {
		delta.Exp(big2, new(big.Int).SetUint64(period), nil)
		diff.Add(diff, delta)
	}

	// try new algo after 60_000
	if blockHeight > 60_000 {
		// reload the diff
		diff = new(big.Int).SetBytes(tailBlock.GetDifficulty())
		d := blockTime - tailBlock.Timestamp - int64(TargetTime/time.Second)
		delta.Div(diff, big.NewInt(2048))
		delta.Mul(delta, big.NewInt(max(1-(d)/10, -99)))
		diff.Add(diff, delta)

		delta.Exp(big2, big.NewInt(int64(blockHeight)/100_000-2), nil)
		diff.Add(diff, delta)
	}

	if diff.Cmp(minimumBigDifficulty) < 0 {
		diff = minimumBigDifficulty
	}

	log.Debugf("New Block Diff: %d", diff)

	return diff
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
