package consensus

import (
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
)

var (
	MinimumDifficulty = big.NewInt(50 * 10) // because my laptop has 50 h/s, I believe you can either
)

func GetNextTarget(block *ngtypes.Block, vault *ngtypes.Vault) *big.Int {
	// algorithm1:
	// diff = max or min(fatherDiff/fatherTime, grandpaDiff/grandpaTime) * targetTime
	//        * 2^(fatherTime - grandpaTime)

	target := new(big.Int).SetBytes(block.Header.Target)

	if !block.Header.IsTail() {
		return target
	}

	diff := new(big.Int).Div(ngtypes.MaxTarget, target)
	// when checkpoint
	if block.Header.Timestamp-vault.Timestamp < int64(ngtypes.TargetTime)*(ngtypes.BlockCheckRound-1) {
		diff = new(big.Int).Add(diff, new(big.Int).Div(diff, big.NewInt(10)))
	}

	if diff.Cmp(MinimumDifficulty) < 0 {
		diff = MinimumDifficulty
	}

	log.Info("New Block Diff:", diff)
	return new(big.Int).Div(ngtypes.MaxTarget, diff)
}
