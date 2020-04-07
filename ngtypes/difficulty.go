package ngtypes

import (
	"math/big"
)

// GetNextTarget is a helper to get next pow block target field
// TODO: add target check into chain
func GetNextTarget(tailBlock *Block, tailBlocksVault *Vault) *big.Int {
	target := new(big.Int).SetBytes(tailBlock.Header.Target)
	if !tailBlock.Header.IsTail() {
		return target
	}

	// when next block is head(checkpoint)
	diff := new(big.Int).Div(MaxTarget, target)
	if tailBlock.Header.Timestamp-tailBlocksVault.Timestamp < int64(TargetTime)*(BlockCheckRound-1) {
		diff = new(big.Int).Add(diff, new(big.Int).Div(diff, big.NewInt(10)))
	}

	if diff.Cmp(MinimumDifficulty) < 0 {
		diff = MinimumDifficulty
	}

	log.Debugf("New Block Diff:", diff)
	return new(big.Int).Div(MaxTarget, diff)
}
