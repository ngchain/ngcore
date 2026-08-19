package blockchain

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func TestFinalityHeight(t *testing.T) {
	round := uint64(ngtypes.BlockCheckRound)

	cases := []struct{ tip, want uint64 }{
		{0, 0},
		{1, 0},
		{round, round - round},   // (round-1)/round*round = 0
		{round + 1, round},       // the checkpoint got built upon
		{2*round + 5, 2 * round}, // mid-round
		{3 * round, 2 * round},   // exactly on a round
	}
	for _, c := range cases {
		if got := finalityHeight(c.tip); got != c.want {
			t.Errorf("finalityHeight(%d) = %d, want %d", c.tip, got, c.want)
		}
	}
}

// TestCheckBlockTargetActualDiffTooLow drives the "actual diff below the
// required diff" branch, which the public paths cannot reach (CheckError
// already enforces the declared diff): the block declares the correct
// difficulty but its pow does not satisfy it
func TestCheckBlockTargetActualDiffTooLow(t *testing.T) {
	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	miner, _ := ngtypes.GenerateKey()

	// a fake parent with a huge difficulty forces a huge next diff
	bigDiff := new(big.Int).Lsh(big.NewInt(1), 40)
	parentTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET) + 16
	parent := ngtypes.NewBareBlock(ngtypes.ZERONET, 1, parentTime, genesis.GetHash(), bigDiff)

	genTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
		ngtypes.NewAddress(miner), ngtypes.GetBlockReward(1), big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}
	if err := parent.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
		t.Fatal(err)
	}
	if err := parent.ToSealed(utils.PackUint64LE(0)); err != nil {
		t.Fatal(err)
	}

	blockTime := parentTime + 16
	correctDiff := ngtypes.GetNextDiff(2, blockTime, parent)
	block := ngtypes.NewBareBlock(ngtypes.ZERONET, 2, blockTime, parent.GetHash(), correctDiff)

	genTx2 := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 2,
		ngtypes.NewAddress(miner), ngtypes.GetBlockReward(2), big.NewInt(0), nil, nil)
	if err := genTx2.Signature(miner); err != nil {
		t.Fatal(err)
	}
	if err := block.ToUnsealing([]*ngtypes.FullTx{genTx2}); err != nil {
		t.Fatal(err)
	}

	// find a nonce whose pow does NOT reach the declared difficulty
	sealed := false
	for n := uint64(0); n < 1_000_000; n++ {
		if err := block.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if block.GetActualDiff().Cmp(correctDiff) < 0 {
			sealed = true
			break
		}
	}
	if !sealed {
		t.Fatal("could not find an under-target nonce")
	}

	err := checkBlockTarget(block, parent)
	if !errors.Is(err, ngtypes.ErrBlockDiffInvalid) {
		t.Fatalf("got %v, want ErrBlockDiffInvalid", err)
	}
}
