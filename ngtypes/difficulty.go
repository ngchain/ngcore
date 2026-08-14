package ngtypes

import (
	"math/big"
	"time"
)

var big2 = big.NewInt(2)

// MinimumDiffOf returns the minimum pow difficulty of the network.
// ZERONET is the local regression network, where any nonce should work,
// so its blocks stay instantly minable
func MinimumDiffOf(network Network) *big.Int {
	if network == ZERONET {
		return big.NewInt(1)
	}

	return minimumBigDifficulty
}

// GetNextDiff is a helper to get next pow block Diff field.
func GetNextDiff(blockHeight uint64, blockTime uint64, tailBlock *FullBlock) *big.Int {
	diff := new(big.Int).SetBytes(tailBlock.BlockHeader.Difficulty)
	if !tailBlock.IsTail() {
		return diff
	}

	if tailBlock.GetTimestamp() < GetGenesisTimestamp(tailBlock.BlockHeader.Network) {
		panic("network havent start yet")
	}
	elapsed := tailBlock.GetTimestamp() - GetGenesisTimestamp(tailBlock.BlockHeader.Network)
	diffTime := int64(elapsed) - int64(tailBlock.GetHeight())*int64(TargetTime/time.Second)
	delta := new(big.Int)
	if diffTime < int64(TargetTime/time.Second)*(-2) {
		delta.Div(diff, big.NewInt(10))
	}

	if diffTime > int64(TargetTime/time.Second)*(+2) {
		delta.Div(diff, big.NewInt(10))
	}

	// reload the diff
	diff = new(big.Int).SetBytes(tailBlock.BlockHeader.Difficulty)
	d := int64(blockTime) - int64(tailBlock.GetTimestamp()) - int64(TargetTime/time.Second)
	delta.Div(diff, big.NewInt(2048))
	delta.Mul(delta, big.NewInt(max(1-(d)/10, -99)))
	diff.Add(diff, delta)

	delta.Exp(big2, big.NewInt(int64(blockHeight)/100_000-2), nil)
	diff.Add(diff, delta)

	minimum := MinimumDiffOf(tailBlock.BlockHeader.Network)
	if diff.Cmp(minimum) < 0 {
		diff = minimum
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
