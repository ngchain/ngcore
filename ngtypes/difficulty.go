package ngtypes

import (
	"math/big"
	"time"
)

// GetNextDiff is a helper to get next pow block Diff field.
func GetNextDiff(tailBlock *Block) *big.Int {
	diff := new(big.Int).SetBytes(tailBlock.GetDifficulty())
	if !tailBlock.IsTail() {
		return diff
	}

	elapsed := tailBlock.Timestamp - GenesisTimestamp
	elapsed = elapsed - int64(tailBlock.GetHeight())*int64(TargetTime/time.Second)
	delta := new(big.Int)
	if elapsed < int64(TargetTime/time.Second)*(-2) {
		delta.Div(diff, big.NewInt(10))
		diff.Add(diff, delta)
	}

	if elapsed > int64(TargetTime/time.Second)*(+2) {
		delta.Div(diff, big.NewInt(10))
		diff.Sub(diff, delta)
	}

	period := (tailBlock.Height + 1) / 1000
	if (tailBlock.Height+1)%1000 == 0 && period > 10 {
		delta.Exp(Big2, new(big.Int).SetUint64(period), nil)
		diff.Add(diff, delta)
	}

	if diff.Cmp(minimumBigDifficulty) < 0 {
		diff = minimumBigDifficulty
	}

	log.Debugf("New Block Diff: %d", diff)

	return diff
}
