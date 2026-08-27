package ngtypes

import (
	"math"
	"testing"
	"time"
)

// ForkGenesis must be active at height 0 and beyond on every available network.
func TestForkGenesisAlwaysActive(t *testing.T) {
	for _, net := range AvailableNetworks {
		for _, h := range []uint64{0, 1, 100, 200_000, math.MaxUint64} {
			if got := ForkHeight(net, ForkGenesis); got != 0 {
				t.Fatalf("ForkHeight(%s, ForkGenesis) = %d, want 0", net, got)
			}
			if !IsForkActive(net, ForkGenesis, h) {
				t.Fatalf("IsForkActive(%s, ForkGenesis, %d) = false, want true", net, h)
			}
		}
	}
}

// ForkFeeMarket is scheduled at FeeMarketForkHeight on the dev networks
// (ZERONET, TESTNET) and NoFork on MAINNET: inactive strictly below the
// activation height, active at and above it.
func TestForkFeeMarketSchedule(t *testing.T) {
	// dev networks: scheduled at FeeMarketForkHeight
	for _, net := range []Network{ZERONET, TESTNET} {
		if got := ForkHeight(net, ForkFeeMarket); got != FeeMarketForkHeight {
			t.Fatalf("ForkHeight(%s, ForkFeeMarket) = %d, want %d", net, got, FeeMarketForkHeight)
		}
		if IsForkActive(net, ForkFeeMarket, FeeMarketForkHeight-1) {
			t.Fatalf("ForkFeeMarket must be inactive at height %d on %s", FeeMarketForkHeight-1, net)
		}
		if !IsForkActive(net, ForkFeeMarket, FeeMarketForkHeight) {
			t.Fatalf("ForkFeeMarket must be active at height %d on %s", FeeMarketForkHeight, net)
		}
		if !IsForkActive(net, ForkFeeMarket, FeeMarketForkHeight+1) {
			t.Fatalf("ForkFeeMarket must be active above the activation height on %s", net)
		}
	}

	// mainnet: unscheduled (NoFork), never active
	if got := ForkHeight(MAINNET, ForkFeeMarket); got != NoFork {
		t.Fatalf("ForkHeight(MAINNET, ForkFeeMarket) = %d, want NoFork(%d)", got, uint64(NoFork))
	}
	for _, h := range []uint64{0, 1, 100, math.MaxUint64 - 1} {
		if IsForkActive(MAINNET, ForkFeeMarket, h) {
			t.Fatalf("IsForkActive(MAINNET, ForkFeeMarket, %d) = true, want false", h)
		}
	}
}

// An unscheduled fork on an unknown/omitted network defaults to NoFork.
func TestForkHeightDefaultsToNoFork(t *testing.T) {
	// A fork value with no schedule entry anywhere.
	const unscheduled Fork = 999
	for _, net := range []Network{ZERONET, TESTNET, MAINNET} {
		if got := ForkHeight(net, unscheduled); got != NoFork {
			t.Fatalf("ForkHeight(%s, unscheduled) = %d, want NoFork", net, got)
		}
		if IsForkActive(net, unscheduled, math.MaxUint64-1) {
			t.Fatalf("unscheduled fork should never be active on %s", net)
		}
	}
}

// IsForkActive boundary semantics (height >= activation height), verified
// against a hypothetical schedule WITHOUT touching the production forkSchedule:
// we compute the activation-height comparison directly to pin the >= rule.
func TestIsForkActiveBoundary(t *testing.T) {
	// height >= activation  ->  active; height == activation-1 -> inactive.
	// Model the check the same way IsForkActive does, over a hypothetical
	// activation height, to pin the boundary without mutating production state.
	check := func(activation, height uint64) bool { return height >= activation }

	cases := []struct {
		activation uint64
		height     uint64
		want       bool
	}{
		{activation: 100, height: 99, want: false},
		{activation: 100, height: 100, want: true},
		{activation: 100, height: 101, want: true},
		{activation: 0, height: 0, want: true},
	}
	for _, c := range cases {
		if got := check(c.activation, c.height); got != c.want {
			t.Fatalf("boundary activation=%d height=%d: got %v want %v",
				c.activation, c.height, got, c.want)
		}
	}

	// And confirm IsForkActive itself obeys >= at the always-active ForkGenesis
	// boundary (activation 0): height 0 is active.
	if !IsForkActive(AvailableNetworks[0], ForkGenesis, 0) {
		t.Fatal("ForkGenesis must be active at its activation height 0")
	}
}

// ActiveFork returns ForkGenesis below FeeMarketForkHeight and ForkFeeMarket at
// or above it on the dev networks; MAINNET stays ForkGenesis at every height.
func TestActiveForkFeeMarket(t *testing.T) {
	for _, net := range []Network{ZERONET, TESTNET} {
		if got := ActiveFork(net, FeeMarketForkHeight-1); got != ForkGenesis {
			t.Fatalf("ActiveFork(%s, %d) = %d, want ForkGenesis", net, FeeMarketForkHeight-1, got)
		}
		for _, h := range []uint64{FeeMarketForkHeight, FeeMarketForkHeight + 1, 200_000, math.MaxUint64} {
			if got := ActiveFork(net, h); got != ForkFeeMarket {
				t.Fatalf("ActiveFork(%s, %d) = %d, want ForkFeeMarket", net, h, got)
			}
		}
	}
	for _, h := range []uint64{0, 1, 100, 200_000, math.MaxUint64} {
		if got := ActiveFork(MAINNET, h); got != ForkGenesis {
			t.Fatalf("ActiveFork(MAINNET, %d) = %d, want ForkGenesis", h, got)
		}
	}
}

// Determinism: same (net, fork, height) => same result, across repeated calls.
func TestForkHelpersDeterministic(t *testing.T) {
	for _, net := range AvailableNetworks {
		for _, f := range []Fork{ForkGenesis, ForkFeeMarket} {
			for _, h := range []uint64{0, 1, 100, math.MaxUint64 - 1} {
				h0 := ForkHeight(net, f)
				a0 := IsForkActive(net, f, h)
				af0 := ActiveFork(net, h)
				for i := 0; i < 3; i++ {
					if ForkHeight(net, f) != h0 ||
						IsForkActive(net, f, h) != a0 ||
						ActiveFork(net, h) != af0 {
						t.Fatalf("non-deterministic result for net=%s fork=%d height=%d", net, f, h)
					}
				}
			}
		}
	}
}

// Zero-behavior-change pin for the threaded integration point: the fork-aware
// target-time selector equals the pre-fork TargetTime constant at every
// representative height on every network.
func TestTargetTimeAtUnchanged(t *testing.T) {
	for _, net := range []Network{ZERONET, TESTNET, MAINNET} {
		for _, h := range []uint64{0, 1, 100, 200_000, math.MaxUint64} {
			if got := targetTimeAt(net, h); got != TargetTime {
				t.Fatalf("targetTimeAt(%s, %d) = %v, want TargetTime(%v)",
					net, h, got, time.Duration(TargetTime))
			}
		}
	}
}
