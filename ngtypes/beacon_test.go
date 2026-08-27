package ngtypes

import (
	"math"
	"testing"
)

// ForkRandomBeacon is active from genesis (RandomBeaconForkHeight == 0) on the
// dev networks (ZERONET, TESTNET), co-scheduled with the fee market and state
// rent, and NoFork on MAINNET.
func TestForkRandomBeaconSchedule(t *testing.T) {
	if RandomBeaconForkHeight != 0 {
		t.Fatalf("RandomBeaconForkHeight = %d, want 0 (active from genesis)", RandomBeaconForkHeight)
	}

	for _, net := range []Network{ZERONET, TESTNET} {
		if got := ForkHeight(net, ForkRandomBeacon); got != RandomBeaconForkHeight {
			t.Fatalf("ForkHeight(%s, ForkRandomBeacon) = %d, want %d", net, got, RandomBeaconForkHeight)
		}
		for _, h := range []uint64{0, 1, 100, math.MaxUint64 - 1} {
			if !IsForkActive(net, ForkRandomBeacon, h) {
				t.Fatalf("ForkRandomBeacon must be active at height %d on %s", h, net)
			}
		}
	}

	// mainnet: unscheduled (NoFork), never active
	if got := ForkHeight(MAINNET, ForkRandomBeacon); got != NoFork {
		t.Fatalf("ForkHeight(MAINNET, ForkRandomBeacon) = %d, want NoFork(%d)", got, uint64(NoFork))
	}
	for _, h := range []uint64{0, 1, 100, math.MaxUint64 - 1} {
		if IsForkActive(MAINNET, ForkRandomBeacon, h) {
			t.Fatalf("IsForkActive(MAINNET, ForkRandomBeacon, %d) = true, want false", h)
		}
	}
}

// BeaconStateKey is a fixed non-empty constant: every node must commit the seed
// at the identical leaf, so it must never silently change.
func TestBeaconStateKeyStable(t *testing.T) {
	if string(BeaconStateKey) != "randao" {
		t.Fatalf("BeaconStateKey = %q, want %q", BeaconStateKey, "randao")
	}
}
