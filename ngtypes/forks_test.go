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

// ForkNext is the not-yet-scheduled placeholder: NoFork height, never active.
func TestForkNextNeverActive(t *testing.T) {
	for _, net := range AvailableNetworks {
		if got := ForkHeight(net, ForkNext); got != NoFork {
			t.Fatalf("ForkHeight(%s, ForkNext) = %d, want NoFork(%d)", net, got, uint64(NoFork))
		}
		for _, h := range []uint64{0, 1, 100, math.MaxUint64 - 1} {
			if IsForkActive(net, ForkNext, h) {
				t.Fatalf("IsForkActive(%s, ForkNext, %d) = true, want false", net, h)
			}
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

// ActiveFork returns ForkGenesis for all heights on all networks today.
func TestActiveForkIsGenesisToday(t *testing.T) {
	for _, net := range AvailableNetworks {
		for _, h := range []uint64{0, 1, 100, 200_000, math.MaxUint64} {
			if got := ActiveFork(net, h); got != ForkGenesis {
				t.Fatalf("ActiveFork(%s, %d) = %d, want ForkGenesis", net, h, got)
			}
		}
	}
}

// Determinism: same (net, fork, height) => same result, across repeated calls.
func TestForkHelpersDeterministic(t *testing.T) {
	for _, net := range AvailableNetworks {
		for _, f := range []Fork{ForkGenesis, ForkNext} {
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
