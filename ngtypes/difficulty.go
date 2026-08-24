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

// GetNextDiff is a helper to get next pow block Diff field. It retargets
// on EVERY block from the parent, so the difficulty tracks hashrate
// continuously instead of only at round boundaries.
func GetNextDiff(blockHeight uint64, blockTime uint64, parentBlock *FullBlock) *big.Int {
	diff := new(big.Int).SetBytes(parentBlock.BlockHeader.Difficulty)

	if parentBlock.GetTimestamp() < GetGenesisTimestamp(parentBlock.BlockHeader.Network) {
		panic("network havent start yet")
	}

	// ethereum-homestead style: diff += diff/2048 * clamp((target-d)/target).
	// The interval d and the target are both in MILLISECONDS (target =
	// TargetTime = 4000ms). This is the crux of the fix: with
	// second-resolution timestamps a monotonic chain forces d >= 1s on every
	// block, so against a sub-second target the step was never positive and
	// difficulty decayed to — and pinned at — the floor no matter the
	// hashrate. Millisecond timestamps let d fall on either side of the
	// multi-second target, so a faster-than-target block RAISES difficulty
	// and the retarget actually tracks hashpower.
	target := int64(TargetTime / time.Millisecond)
	if target < 1 {
		target = 1
	}
	d := int64(blockTime) - int64(parentBlock.GetTimestamp())
	// proportional step, clamped to [-99, +1] * diff/2048; scaling the
	// numerator by target (instead of an integer d/target) keeps sub-target
	// intervals from truncating to zero, so the step stays fine-grained.
	// NOTE the clamp is asymmetric: up to +diff/2048 up, but down to
	// -99*diff/2048 — a miner that stamps blocks far ahead (within the drift
	// window) can push difficulty down faster than honest blocks pull it up.
	factor := target - d
	if factor > target {
		factor = target
	}
	if factor < -99*target {
		factor = -99 * target
	}
	delta := new(big.Int).Mul(diff, big.NewInt(factor))
	delta.Quo(delta, big.NewInt(2048*target))
	diff.Add(diff, delta)

	delta.Exp(big2, big.NewInt(int64(blockHeight)/100_000-2), nil)
	diff.Add(diff, delta)

	minimum := MinimumDiffOf(parentBlock.BlockHeader.Network)
	if diff.Cmp(minimum) < 0 {
		diff = minimum
	}

	log.Debugf("New Block Diff: %d", diff)

	return diff
}
