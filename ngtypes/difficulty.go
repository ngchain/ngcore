package ngtypes

import (
	"math/big"
	"time"
)

var big2 = big.NewInt(2)

// GetNextDiff is a helper to get next pow block Diff field.
func GetNextDiff(blockHeight uint64, blockTime int64, tailBlock *Block) *big.Int {
	diff := new(big.Int).SetBytes(tailBlock.Header.GetDifficulty())
	if !tailBlock.IsTail() {
		return diff
	}

	elapsed := tailBlock.Header.Timestamp - GetGenesisTimestamp(tailBlock.Header.GetNetwork())
	diffTime := elapsed - int64(tailBlock.Header.GetHeight())*int64(TargetTime/time.Second)
	delta := new(big.Int)
	if diffTime < int64(TargetTime/time.Second)*(-2) {
		delta.Div(diff, big.NewInt(10))
	}

	if diffTime > int64(TargetTime/time.Second)*(+2) {
		delta.Div(diff, big.NewInt(10))
	}

	// reload the diff
	diff = new(big.Int).SetBytes(tailBlock.Header.GetDifficulty())
	d := blockTime - tailBlock.Header.GetTimestamp() - int64(TargetTime/time.Second)
	delta.Div(diff, big.NewInt(2048))
	delta.Mul(delta, big.NewInt(max(1-(d)/10, -99)))
	diff.Add(diff, delta)

	delta.Exp(big2, big.NewInt(int64(blockHeight)/100_000-2), nil)
	diff.Add(diff, delta)

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
