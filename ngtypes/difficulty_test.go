package ngtypes_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

func TestMinimumDiffOf(t *testing.T) {
	if got := ngtypes.MinimumDiffOf(ngtypes.ZERONET); got.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("ZERONET minimum diff = %s, want 1 (instant regression mining)", got)
	}

	for _, network := range []ngtypes.Network{ngtypes.TESTNET, ngtypes.MAINNET} {
		got := ngtypes.MinimumDiffOf(network)
		if got.Cmp(big.NewInt(1)) <= 0 {
			t.Fatalf("network %s minimum diff = %s, must be a real pow bound", network, got)
		}
	}
}

// diffTestBlock builds a block header shell carrying just what
// GetNextDiff reads: network, height, timestamp and declared difficulty
func diffTestBlock(network ngtypes.Network, height uint64, blockTime uint64, diff *big.Int) *ngtypes.FullBlock {
	return ngtypes.NewBareBlock(network, height, blockTime, make([]byte, ngtypes.HashSize), diff)
}

func TestGetNextDiffKeepsDiffOffTail(t *testing.T) {
	genesisTime := ngtypes.GetGenesisTimestamp(ngtypes.ZERONET)

	// a non-tail block (height 5, with BlockCheckRound 10) must not retarget
	parent := diffTestBlock(ngtypes.ZERONET, 5, genesisTime+5*16, big.NewInt(7))
	if parent.IsTail() {
		t.Fatal("height 5 must not be a tail with BlockCheckRound 10")
	}

	got := ngtypes.GetNextDiff(6, genesisTime+6*16, parent)
	if got.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("off-tail next diff = %s, want unchanged 7", got)
	}
}

func TestGetNextDiffClampsToMinimum(t *testing.T) {
	tailHeight := uint64(ngtypes.BlockCheckRound - 1) // 9: the retarget point

	for _, network := range []ngtypes.Network{ngtypes.ZERONET, ngtypes.TESTNET} {
		genesisTime := ngtypes.GetGenesisTimestamp(network)
		minimum := ngtypes.MinimumDiffOf(network)

		// a tail block already at the minimum, with wildly slow blocks:
		// the retarget must never go below the network minimum
		slowTime := genesisTime + tailHeight*1000
		parent := diffTestBlock(network, tailHeight, slowTime, minimum)
		if !parent.IsTail() {
			t.Fatalf("height %d must be a tail", tailHeight)
		}

		got := ngtypes.GetNextDiff(tailHeight+1, slowTime+1000, parent)
		if got.Cmp(minimum) < 0 {
			t.Fatalf("network %s: retargeted diff %s fell below the minimum %s",
				network, got, minimum)
		}
	}
}

func TestGetNextDiffRetargetsUpOnFastBlocks(t *testing.T) {
	// blocks coming much faster than the target time must not LOWER the
	// difficulty on the retarget point
	network := ngtypes.TESTNET
	genesisTime := ngtypes.GetGenesisTimestamp(network)
	tailHeight := uint64(ngtypes.BlockCheckRound - 1)

	start := new(big.Int).Mul(ngtypes.MinimumDiffOf(network), big.NewInt(4))
	// all 9 blocks arrived within one second of the genesis
	parent := diffTestBlock(network, tailHeight, genesisTime+1, start)

	next := ngtypes.GetNextDiff(tailHeight+1, genesisTime+2, parent)
	if next.Cmp(start) < 0 {
		t.Fatalf("fast blocks lowered the diff: %s -> %s", start, next)
	}
}

func TestGetNextDiffTargetTimeSanity(t *testing.T) {
	// the retarget window math assumes a sub-minute block target
	if ngtypes.TargetTime <= 0 || ngtypes.TargetTime > time.Minute {
		t.Fatalf("unexpected TargetTime %s", ngtypes.TargetTime)
	}
}
