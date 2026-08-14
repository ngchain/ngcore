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

	// ethereum-homestead style: diff += diff/2048 * max(1 - d/target, -99),
	// with the gap normalized by the TARGET TIME so the same formula
	// holds for 1-second blocks (d == target keeps the diff stable)
	target := int64(TargetTime / time.Second)
	if target < 1 {
		target = 1
	}
	d := int64(blockTime) - int64(tailBlock.GetTimestamp())
	delta := new(big.Int).Div(diff, big.NewInt(2048))
	delta.Mul(delta, big.NewInt(max(1-d/target, -99)))
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
