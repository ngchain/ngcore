package ngtypes

import "math"

// Fork is an ordered enum of named protocol versions (rulesets). A higher
// Fork value is a strictly newer ruleset that supersedes all lower ones. The
// active ruleset at a block is chosen IMPLICITLY BY HEIGHT (like Ethereum's
// hardcoded fork blocks): no header field carries the fork, so scheduling a
// fork never collides with future header-format changes. The Network byte
// already selects the network; the activation schedule is per-network config
// (see ForkHeight).
//
// This framework is PURE ADDITIVE: ForkGenesis is the CURRENT ruleset, active
// from genesis on every network, so nothing about today's consensus changes.
// Every FUTURE consensus change gets its own Fork value plus a scheduled
// activation height, and the gated code reads IsForkActive / a fork-aware
// parameter selector rather than a bare constant.
type Fork uint16

const (
	// ForkGenesis is the ruleset in force today. It is always active (its
	// activation height is 0 on every network), so all existing consensus
	// behavior is unchanged. Do NOT renumber it.
	ForkGenesis Fork = 0

	// ForkNext is a placeholder for the next, not-yet-scheduled upgrade. It
	// demonstrates the pattern: it exists in the enum but ForkHeight returns
	// the NoFork sentinel for it on every network, so IsForkActive is false at
	// every height until a maintainer schedules a real activation height. When
	// a concrete upgrade is designed, rename this (or add a new value after it)
	// and give it a height in forkSchedule below.
	ForkNext Fork = 1
)

// NoFork is the "never active" sentinel activation height: a fork whose
// ForkHeight is NoFork is disabled, since no block height can be >= MaxUint64.
const NoFork uint64 = math.MaxUint64

// forkSchedule is the per-network fork activation table.
//
//	=== THIS IS WHERE A MAINTAINER SCHEDULES A FUTURE FORK ===
//
// To activate a fork on a network, set its entry to the target block height
// (one-line change). ForkGenesis is intentionally ABSENT and handled as 0 by
// ForkHeight (it is always active); a fork not listed for a network defaults
// to NoFork (never active on that network). Networks may schedule the same
// fork at different heights, or omit it entirely.
//
// Keep this a static, deterministic literal: no time, no rand, no runtime
// mutation. It is read on every block validation.
var forkSchedule = map[Network]map[Fork]uint64{
	ZERONET: {
		ForkNext: NoFork, // not scheduled yet
	},
	TESTNET: {
		ForkNext: NoFork, // not scheduled yet
	},
	MAINNET: {
		ForkNext: NoFork, // not scheduled yet
	},
}

// ForkHeight returns the activation height of fork f on network net.
//
//	ForkGenesis  => 0    (always active, on every network)
//	scheduled    => the height in forkSchedule
//	unscheduled  => NoFork (never active)
//
// It is pure and deterministic: same (net, f) always yields the same height.
func ForkHeight(net Network, f Fork) uint64 {
	if f == ForkGenesis {
		return 0
	}
	if netSchedule, ok := forkSchedule[net]; ok {
		if h, ok := netSchedule[f]; ok {
			return h
		}
	}
	return NoFork
}

// IsForkActive reports whether fork f is active at the given block height on
// network net, i.e. height >= ForkHeight(net, f). ForkGenesis is always active
// (its height is 0). A fork scheduled at NoFork is NEVER active — including at
// height == MaxUint64 — so the sentinel is unconditionally disabled rather than
// relying on the theoretical unreachability of that height.
func IsForkActive(net Network, f Fork, height uint64) bool {
	h := ForkHeight(net, f)
	if h == NoFork {
		return false
	}
	return height >= h
}

// ActiveFork returns the highest fork active at the given height on net — the
// "current ruleset" at that block. Today it returns ForkGenesis for every
// height on every network (nothing else is scheduled). It walks the ordered
// fork values upward, stopping at the first inactive one, so it stays correct
// as new forks are appended in order.
func ActiveFork(net Network, height uint64) Fork {
	active := ForkGenesis
	for f := ForkGenesis + 1; f <= ForkNext; f++ {
		if IsForkActive(net, f, height) {
			active = f
		}
	}
	return active
}
