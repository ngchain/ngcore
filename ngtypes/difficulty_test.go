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

func TestGetNextDiffRetargetsEveryBlock(t *testing.T) {
	// difficulty now retargets on EVERY block, not only at round
	// boundaries — a NON-tail height must still track the interval
	network := ngtypes.TESTNET
	genesisTime := ngtypes.GetGenesisTimestamp(network)
	start := new(big.Int).Mul(ngtypes.MinimumDiffOf(network), big.NewInt(4))

	parent := diffTestBlock(network, 5, genesisTime+5*16, start)
	if parent.IsTail() {
		t.Fatal("height 5 must not be a tail with BlockCheckRound 10")
	}

	// a sub-target interval (16ms << 1000ms target) raises the diff
	fast := ngtypes.GetNextDiff(6, genesisTime+5*16+16, parent)
	if fast.Cmp(start) <= 0 {
		t.Fatalf("fast off-tail block did not raise diff: %s -> %s", start, fast)
	}

	// a slower-than-target interval lowers it
	slow := ngtypes.GetNextDiff(6, genesisTime+5*16+5000, parent)
	if slow.Cmp(start) >= 0 {
		t.Fatalf("slow off-tail block did not lower diff: %s -> %s", start, slow)
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

// TestGetNextDiffTracksHashrateOverAChain is the regression guard for the
// pinned-difficulty flaw: with second-resolution timestamps a monotonic
// chain forced every interval >= the 1s target, so difficulty decayed to
// the floor and never tracked hashrate. With millisecond timestamps a
// sustained run of faster-than-target blocks must climb well above the
// minimum, and a slower run must fall below it.
func TestGetNextDiffTracksHashrateOverAChain(t *testing.T) {
	network := ngtypes.TESTNET
	genesisTime := ngtypes.GetGenesisTimestamp(network)
	minimum := ngtypes.MinimumDiffOf(network)

	// intervals are in milliseconds; TargetTime is 1000ms
	run := func(interval uint64) *big.Int {
		diff := new(big.Int).Mul(minimum, big.NewInt(8))
		ts := genesisTime
		for h := uint64(1); h <= 1000; h++ {
			parent := diffTestBlock(network, h-1, ts, diff)
			ts += interval
			diff = ngtypes.GetNextDiff(h, ts, parent)
		}
		return diff
	}

	fast := run(50)   // 50ms << 1000ms target
	slow := run(5000) // 5000ms >> target

	if fast.Cmp(slow) <= 0 {
		t.Fatalf("difficulty did not track hashrate over a chain: fast=%s slow=%s", fast, slow)
	}
	if fast.Cmp(minimum) <= 0 {
		t.Fatalf("sustained fast blocks did not raise diff above the floor %s: %s", minimum, fast)
	}
}
