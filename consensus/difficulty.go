package consensus

import (
	"github.com/ngin-network/ngcore/ngtypes"
	"math/big"
)

var (
	MinimumDifficulty = big.NewInt(50 * 10) // because my laptop has 50 h/s, I believe you can either
)

func (c *Consensus) getNextTarget(block *ngtypes.Block, vault *ngtypes.Vault) *big.Int {
	// algorithm1:
	// diff = max or min(fatherDiff/fatherTime, grandpaDiff/grandpaTime) * targetTime
	//        * 2^(fatherTime - grandpaTime)

	target := new(big.Int).SetBytes(block.Header.Target)
	if !block.Header.IsCheckpoint() {
		return target
	}

	diff := new(big.Int).Div(ngtypes.MaxTarget, target)
	// when checkpoint
	if block.Header.Timestamp-vault.Timestamp < int64(ngtypes.TargetTime)*(ngtypes.CheckRound-1) {
		diff = new(big.Int).Add(diff, new(big.Int).Div(diff, big.NewInt(10)))
	}

	if diff.Cmp(MinimumDifficulty) < 0 {
		diff = MinimumDifficulty
	}

	log.Info("New Block Diff:", diff)
	return new(big.Int).Div(ngtypes.MaxTarget, diff)
}
