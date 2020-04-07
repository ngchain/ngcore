package ngtypes

import (
	"math/big"
)

var (
	// MinimumDifficulty is the minimum of pow difficulty because my laptop has 50 h/s, I believe you can either
	MinimumDifficulty = big.NewInt(50 * 10)
)

// GetNextTarget is a helper to get next pow block target field
// TODO: add target check into chain
func GetNextTarget(block *Block, nextBlockBasedVault *Vault) *big.Int {
	target := new(big.Int).SetBytes(block.Header.Target)

	if !block.Header.IsTail() {
		return target
	}

	// when next block is head(checkpoint)
	diff := new(big.Int).Div(MaxTarget, target)
	if block.Header.Timestamp-nextBlockBasedVault.Timestamp < int64(TargetTime)*(BlockCheckRound-1) {
		diff = new(big.Int).Add(diff, new(big.Int).Div(diff, big.NewInt(10)))
	}

	if diff.Cmp(MinimumDifficulty) < 0 {
		diff = MinimumDifficulty
	}

	log.Debugf("New Block Diff:", diff)
	return new(big.Int).Div(MaxTarget, diff)
}
