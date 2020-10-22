package ngtypes_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

func TestDiffifultyAlgo(t *testing.T) {
	tailBlock := &ngtypes.Block{
		Timestamp:  ngtypes.GenesisTimestamp + 9*int64(ngtypes.TargetTime/time.Second) - 129,
		Height:     9, // tail
		Difficulty: ngtypes.GetGenesisBlock(ngtypes.NetworkType_TESTNET).GetDifficulty(),
	}

	genesisDiff := new(big.Int).SetBytes(ngtypes.GetGenesisBlock(ngtypes.NetworkType_TESTNET).GetDifficulty())
	diff := ngtypes.GetNextDiff(tailBlock)
	if diff.Cmp(genesisDiff) <= 0 {
		t.Errorf("diff %d should be higher than %d", diff, genesisDiff)
	}

	nextTailBlock := &ngtypes.Block{
		Timestamp:  ngtypes.GenesisTimestamp + 19*int64(ngtypes.TargetTime/time.Second) + 129,
		Height:     19, // tail
		Difficulty: diff.Bytes(),
	}

	nextDiff := ngtypes.GetNextDiff(nextTailBlock)
	if nextDiff.Cmp(diff) >= 0 {
		t.Errorf("diff %d should be lower than %d", nextDiff, diff)
	}
}
