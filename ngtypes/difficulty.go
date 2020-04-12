package ngtypes

import (
	"math/big"
	"time"
)

// GetNextTarget is a helper to get next pow block target field
func GetNextTarget(tailBlock *Block, tailBlocksVault *Vault) *big.Int {
	target := new(big.Int).SetBytes(tailBlock.Header.Target)
	if !tailBlock.Header.IsTail() {
		return target
	}

	// when next block is head(checkpoint)
	diff := new(big.Int).Div(MaxTarget, target)
	elapsed := tailBlock.Header.Timestamp - tailBlocksVault.Timestamp
	if elapsed < int64(TargetTime/time.Second)*(BlockCheckRound-2) {
		diff = new(big.Int).Add(diff, new(big.Int).Div(diff, big.NewInt(10)))
	}

	if elapsed > int64(TargetTime/time.Second)*(BlockCheckRound+2) {
		diff = new(big.Int).Sub(diff, new(big.Int).Div(diff, big.NewInt(10)))
	}

	if diff.Cmp(MinimumDifficulty) < 0 {
		diff = MinimumDifficulty
	}

	log.Debugf("New Block Diff:", diff)
	return new(big.Int).Div(MaxTarget, diff)
}
