package ngtypes

import (
	"math"
	"math/big"
	"testing"
)

// ForkStateRent is active from genesis (StateRentForkHeight == 0) on the dev
// networks (ZERONET, TESTNET), co-active with the fee market, and NoFork on
// MAINNET: active at every height on the dev networks, never active on MAINNET.
func TestForkStateRentSchedule(t *testing.T) {
	// co-active with the fee market from genesis: same activation height, both 0
	if StateRentForkHeight != FeeMarketForkHeight {
		t.Fatalf("StateRentForkHeight (%d) must equal FeeMarketForkHeight (%d): both active from genesis",
			StateRentForkHeight, FeeMarketForkHeight)
	}
	if StateRentForkHeight != 0 {
		t.Fatalf("StateRentForkHeight = %d, want 0 (active from genesis)", StateRentForkHeight)
	}

	for _, net := range []Network{ZERONET, TESTNET} {
		if got := ForkHeight(net, ForkStateRent); got != StateRentForkHeight {
			t.Fatalf("ForkHeight(%s, ForkStateRent) = %d, want %d", net, got, StateRentForkHeight)
		}
		for _, h := range []uint64{0, 1, 100, math.MaxUint64 - 1} {
			if !IsForkActive(net, ForkStateRent, h) {
				t.Fatalf("ForkStateRent must be active at height %d on %s", h, net)
			}
		}
	}

	// mainnet: unscheduled (NoFork), never active
	if got := ForkHeight(MAINNET, ForkStateRent); got != NoFork {
		t.Fatalf("ForkHeight(MAINNET, ForkStateRent) = %d, want NoFork(%d)", got, uint64(NoFork))
	}
	for _, h := range []uint64{0, 1, 100, math.MaxUint64 - 1} {
		if IsForkActive(MAINNET, ForkStateRent, h) {
			t.Fatalf("IsForkActive(MAINNET, ForkStateRent, %d) = true, want false", h)
		}
	}
}

// ActiveFork returns ForkStateRent — the highest scheduled fork — at every
// height on the dev networks, since both later forks are active from genesis.
// MAINNET never reaches ForkStateRent (it is NoFork there).
func TestActiveForkStateRent(t *testing.T) {
	for _, net := range []Network{ZERONET, TESTNET} {
		for _, h := range []uint64{0, 1, StateRentForkHeight, StateRentForkHeight + 1, 200_000, math.MaxUint64} {
			if got := ActiveFork(net, h); got != ForkStateRent {
				t.Fatalf("ActiveFork(%s, %d) = %d, want ForkStateRent", net, h, got)
			}
		}
	}
	for _, h := range []uint64{0, 1, StateRentForkHeight, math.MaxUint64} {
		if got := ActiveFork(MAINNET, h); got == ForkStateRent {
			t.Fatalf("ActiveFork(MAINNET, %d) = ForkStateRent, want it never reached on mainnet", h)
		}
	}
}

// The escrow address is the fixed all-0x01 32-byte address, distinct from the
// all-zero GenesisAddress, and round-trips through its base58 constant.
func TestStorageDepositEscrowAddress(t *testing.T) {
	if StorageDepositEscrow == (Address{}) {
		t.Fatal("escrow address must not be the all-zero genesis address")
	}
	if StorageDepositEscrow == GenesisAddress {
		t.Fatal("escrow address must be distinct from GenesisAddress")
	}
	for i, b := range StorageDepositEscrow {
		if b != 0x01 {
			t.Fatalf("escrow address byte %d = %#x, want 0x01", i, b)
		}
	}
	if got := StorageDepositEscrow.BS58(); got != StorageDepositEscrowBase58 {
		t.Fatalf("escrow base58 round trip: %q != %q", got, StorageDepositEscrowBase58)
	}
}

// DepositPerByte is the documented 1e12 pico-NG (1e-6 NG) per stored byte.
func TestDepositPerByte(t *testing.T) {
	if DepositPerByte.Cmp(big.NewInt(1_000_000_000_000)) != 0 {
		t.Fatalf("DepositPerByte = %s, want 1e12", DepositPerByte)
	}
	if DepositPerByte.Sign() <= 0 {
		t.Fatal("DepositPerByte must be positive")
	}
}
