package ngtypes

import (
	"math/big"
	"time"
)

// GetNextTarget is a helper to get next pow block target field.
func GetNextTarget(tailBlock *Block) *big.Int {
	target := new(big.Int).SetBytes(tailBlock.Header.Target)

	if !tailBlock.IsTail() {
		return target
	}

	// when next block is head(checkpoint)
	diff := new(big.Int).Div(maxTarget, target)
	elapsed := int64(uint64(tailBlock.Header.Timestamp) - tailBlock.GetHeight()*uint64(TargetTime/time.Second))

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

	return new(big.Int).Div(maxTarget, diff)
}
