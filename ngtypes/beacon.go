package ngtypes

// Native randomness beacon (RANDAO), gated by ForkRandomBeacon.
//
// The chain accumulates a single 32-byte seed, advanced once per post-genesis
// block from that block's revealed commit-reveal salts. The salts are the
// bias-resistant term: each was committed CommitWindow blocks before it could be
// revealed, so no one choosing to reveal now can pick its value. The seed is a
// committed leaf of the StateRoot (statetrie DomainBeacon), and contracts read
// the PARENT block's finalized seed via the crypto.random host op — so the
// current block's producer cannot grind the value its own block yields.
//
// The accumulation itself lives in ngstate (updateBeacon); this file only pins
// the fixed leaf/bucket key so the trie, the proof path, and the snapshot all
// agree on where the single seed lives.

// BeaconStateKey is the fixed key of the single beacon entry, in both the
// state:beacon bucket and — domain-tagged with statetrie.DomainBeacon — the
// state-commitment leaf path. It is a constant (the beacon is one global value),
// so every node commits the seed at the identical leaf.
var BeaconStateKey = []byte("randao")
