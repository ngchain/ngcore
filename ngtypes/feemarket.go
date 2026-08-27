package ngtypes

import (
	"math/big"

	"github.com/c0mm4nd/rlp"
)

// Burn-only dynamic base fee (ForkFeeMarket).
//
// A consensus-computed per-byte BaseFee is carried in every header. Post-fork,
// each non-generate tx must pay Fee >= BaseFee * len(rlp(tx)). The whole Fee is
// still fully BURNED by the existing chargeFrom path — there is no miner tip and
// the coinbase/generate minting is untouched — so the base fee only prices
// congestion and floors the fee, preserving the chain's deflation.
//
// The update rule mirrors EIP-1559 in pure integer big.Int math (no floats,
// maps, time or rand), so every node derives the identical next base fee.
var (
	// MinBaseFee is the per-byte floor the base fee never falls below. It equals
	// the pre-fork relay floor (DefaultMinFeePerByte = 1e10) for smooth
	// continuity: pre-fork and genesis blocks all carry exactly MinBaseFee.
	MinBaseFee = big.NewInt(10_000_000_000) // 1e10

	// BaseFeeTargetBytes is the per-block used-bytes target the base fee steers
	// toward: half the block size cap. Above it the fee rises, below it falls.
	BaseFeeTargetBytes = uint64(MaxBlockBytes / 2)
)

// BaseFeeMaxChangeDenom caps the per-block base-fee move to 1/denom of the
// parent's base fee (EIP-1559's 12.5% at denom 8).
const BaseFeeMaxChangeDenom = 8

// NextBaseFee returns the base fee the CHILD block (parentHeight+1) must carry.
//
//   - If the child is PRE-fork it is the constant MinBaseFee (the field is
//     present from genesis but not yet updated).
//   - If the child is POST-fork it is the EIP-1559 integer update from the
//     parent's base fee and used bytes, clamped to at most a 1/denom move per
//     block and floored at MinBaseFee.
//
// parentUsedBytes is Σ len(rlp(tx)) over the parent's NON-generate txs (see
// BlockUsedBytes). Pure and deterministic in (net, parentHeight, parentBaseFee,
// parentUsedBytes).
func NextBaseFee(net Network, parentHeight uint64, parentBaseFee *big.Int, parentUsedBytes uint64) *big.Int {
	childHeight := parentHeight + 1

	// pre-fork child: the base fee is not yet updated, it is pinned at the floor
	if !IsForkActive(net, ForkFeeMarket, childHeight) {
		return new(big.Int).Set(MinBaseFee)
	}

	// defensive: a missing/zero parent base fee snaps to the floor before the
	// proportional step so post-fork math always starts from a sane value
	parent := parentBaseFee
	if parent == nil || parent.Sign() <= 0 {
		parent = MinBaseFee
	}

	target := BaseFeeTargetBytes
	denom := big.NewInt(BaseFeeMaxChangeDenom)
	targetBig := new(big.Int).SetUint64(target)

	switch {
	case parentUsedBytes == target:
		return clampMinBaseFee(new(big.Int).Set(parent))

	case parentUsedBytes > target:
		// delta = max(1, parent * (used-target) / target / denom)
		delta := new(big.Int).SetUint64(parentUsedBytes - target)
		delta.Mul(delta, parent)
		delta.Div(delta, targetBig)
		delta.Div(delta, denom)
		if delta.Sign() == 0 {
			delta.SetInt64(1)
		}
		return clampMinBaseFee(new(big.Int).Add(parent, delta))

	default: // parentUsedBytes < target
		// delta = parent * (target-used) / target / denom
		delta := new(big.Int).SetUint64(target - parentUsedBytes)
		delta.Mul(delta, parent)
		delta.Div(delta, targetBig)
		delta.Div(delta, denom)
		return clampMinBaseFee(new(big.Int).Sub(parent, delta))
	}
}

// clampMinBaseFee floors x at MinBaseFee (returning a fresh big.Int when it
// clamps so callers never alias MinBaseFee).
func clampMinBaseFee(x *big.Int) *big.Int {
	if x.Cmp(MinBaseFee) < 0 {
		return new(big.Int).Set(MinBaseFee)
	}
	return x
}

// BlockUsedBytes is the fee-market "used gas" of a block: Σ len(rlp(tx)) over
// its NON-generate txs. Generates (the miner reward + uncle rewards) are system
// mints that pay no base fee, so they are excluded. It drives NextBaseFee for
// the child block.
func BlockUsedBytes(block *FullBlock) uint64 {
	var used uint64
	for _, tx := range block.Txs {
		if tx.Type == GenerateTx {
			continue
		}
		raw, err := rlp.EncodeToBytes(tx)
		if err != nil {
			// a block that reached this point already rlp-encoded whole; an
			// un-encodable tx cannot exist in a valid block, but stay defensive
			continue
		}
		used += uint64(len(raw))
	}
	return used
}
