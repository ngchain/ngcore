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

	target := new(big.Int).Div(MaxTarget, diff)

	// when next block is head(checkpoint)
	diff = new(big.Int).Div(MaxTarget, target)
	elapsed := int64(tailBlock.Timestamp - GenesisTimestamp - int64(tailBlock.GetHeight())*int64(TargetTime/time.Second))

	if elapsed < int64(TargetTime/time.Second)*(BlockCheckRound-2) {
		diff = new(big.Int).Add(diff, new(big.Int).Div(diff, big.NewInt(10)))
	}

	if elapsed > int64(TargetTime/time.Second)*(BlockCheckRound+2) {
		diff = new(big.Int).Sub(diff, new(big.Int).Div(diff, big.NewInt(10)))
	}

	if diff.Cmp(minimumBigDifficulty) < 0 {
		diff = minimumBigDifficulty
	}

	log.Debugf("New Block Diff: %d", diff)

	return diff
}
