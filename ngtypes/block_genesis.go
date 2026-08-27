package ngtypes

import (
	"github.com/ngchain/ngcore/statetrie"
)

var genesisBlock *FullBlock

// genesisStateRoot computes the POST-state root of the genesis block with
// statetrie's in-memory store, WITHOUT importing ngstate (which would form
// an import cycle). It replays exactly what ngstate does when applying the
// genesis block to a fresh, empty state db:
//
//   - the genesis sheet carries no balances/contracts/keys (no premine), so
//     the tree starts empty;
//   - the single genesis generate tx credits GenesisAddress the height-0
//     block reward — a balance leaf. Its signature is all-zero (scheme
//     0x00), so it registers NO public key: there is no key leaf.
//
// A cross-check test in ngstate applies the real genesis block to a fresh
// state db and asserts ngstate.StateRoot == this root, pinning the two
// implementations together.
func genesisStateRoot(network Network) []byte {
	store := statetrie.NewMemStore()

	reward := GetBlockReward(0)
	// mirror ngstate setBalance: value is the balance's minimal big-endian
	// bytes; a zero balance would be an absent leaf (not the case here)
	if reward.Sign() != 0 {
		path := statetrie.LeafPath(statetrie.DomainBalance, GenesisAddress[:])
		_ = statetrie.Update(store, path, statetrie.ValueHash(reward.Bytes()))
	}

	return statetrie.Root(store)
}

// GetGenesisBlock will return a complete sealed GenesisBlock.
func GetGenesisBlock(network Network) *FullBlock {
	txs := []*FullTx{
		GetGenesisGenerateTx(network),
	}

	// memoize per network: caching on nil alone froze the singleton to the
	// FIRST caller's network, so GetGenesisBlock(TESTNET) after ZERONET
	// returned the ZERONET block (GetGenesisGenerateTx already re-checks
	// this way)
	if genesisBlock == nil || genesisBlock.Network != network {
		txTrie := NewTxTrie(txs)
		genesisBlock = NewBlock(
			network,
			0,
			GetGenesisTimestamp(network),

			make([]byte, HashSize),
			txTrie.TrieRoot(),
			CalcWitnessRoot(txs, nil),
			MinimumDiffOf(network).Bytes(), // this is a number, dont put any padding on
			GetGenesisBlockNonce(network),
			txs,
		)
		genesisBlock.SetCoinbase(GenesisAddress)
		// the genesis header commits to its own post-state root (folded into
		// the pow preimage and the block hash), just like every later block
		genesisBlock.BlockHeader.StateRoot = genesisStateRoot(network)
		// genesis carries the base-fee floor; every pre-fork block equals this
		// (NextBaseFee returns MinBaseFee pre-fork), so the chain-layer base-fee
		// check holds from genesis onward
		genesisBlock.BlockHeader.BaseFee = MinBaseFee.Bytes()
		genesisBlock.GetHash()
	}

	return genesisBlock
}
