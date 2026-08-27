package ngstate

import (
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// Native randomness beacon (RANDAO), gated by ForkRandomBeacon.
//
// A single 32-byte seed is advanced once per post-genesis block from that
// block's revealed commit-reveal salts, folded with the prior seed and the
// parent hash. It is written to the state:beacon bucket AND committed as a
// DomainBeacon leaf of the StateRoot, and reorg-unwound through its own
// height-keyed pre-image changeset (archive nodes; non-archive nodes fall back
// to full replay, which regenerates it). Contracts read it via crypto.random.

// beaconDomain separates the beacon accumulation hash from every other use of
// utils.Hash256, so a seed can never collide with a leaf / tx / commit hash.
var beaconDomain = []byte("ngcore/randao")

// getBeacon reads the current accumulated beacon seed — the value contracts see
// while THIS block executes, i.e. the parent block's finalized seed. Absent
// (pre-fork / genesis) reads as 32 zero bytes, a stable well-defined seed.
func getBeacon(txn *bbolt.Tx) []byte {
	seed := make([]byte, ngtypes.HashSize)
	if b := txn.Bucket(storage.BeaconBucketName); b != nil {
		if v := b.Get(ngtypes.BeaconStateKey); v != nil {
			copy(seed, v)
		}
	}
	return seed
}

// setBeacon writes the seed to its bucket AND its state-commitment leaf, after
// recording the pre-image under the block height (archive only) so a reorg
// unwind restores it in lock-step with the balances.
func setBeacon(txn *bbolt.Tx, cs *changeset, seed []byte) {
	cs.recordBeacon(txn) // no-op on a nil recorder
	_ = txn.Bucket(storage.BeaconBucketName).Put(ngtypes.BeaconStateKey, seed)
	trieSetBeacon(txn, seed)
}

// updateBeacon folds this block's revealed entropy into the beacon, once, at the
// END of Upgrade — so during the block's tx execution contracts still read the
// PARENT's seed, the RANDAO-correct value the current producer cannot grind.
//
//	seed(h) = Hash256( beaconDomain ‖ seed(h-1) ‖ prevBlockHash ‖ saltsHash ‖ heightLE )
//	saltsHash = Hash256( ‖ tx.Salt for every reveal tx in block order )
//
// The reveal salts are the bias-resistant term: each was committed CommitWindow
// blocks earlier, so no one revealing now can pick its value; the producer's only
// residual influence is which reveals to include (the standard RANDAO last-actor
// bias). prevBlockHash keeps the seed advancing even in a reveal-less block, at
// the cost of the parent miner's nonce grind on that weaker term.
//
// Genesis (height 0) is skipped so the genesis post-state root — cross-checked
// against ngtypes.genesisStateRoot — stays beacon-free; the first seed is at h=1.
func updateBeacon(txn *bbolt.Tx, cs *changeset, block *ngtypes.FullBlock) {
	if !ngtypes.IsForkActive(block.Network, ngtypes.ForkRandomBeacon, block.GetHeight()) {
		return
	}
	if block.GetHeight() == 0 {
		return
	}

	prev := getBeacon(txn)

	// fold the reveal salts (bias-resistant, pre-committed entropy), in tx order
	saltAcc := make([]byte, 0)
	for _, tx := range block.Txs {
		if len(tx.Salt) != 0 {
			saltAcc = append(saltAcc, tx.Salt...)
		}
	}
	saltsHash := utils.Hash256(saltAcc)

	buf := make([]byte, 0, len(beaconDomain)+ngtypes.HashSize*3+8)
	buf = append(buf, beaconDomain...)
	buf = append(buf, prev...)
	buf = append(buf, block.BlockHeader.PrevBlockHash...)
	buf = append(buf, saltsHash...)
	buf = append(buf, utils.PackUint64LE(block.GetHeight())...)

	setBeacon(txn, cs, utils.Hash256(buf))
}
