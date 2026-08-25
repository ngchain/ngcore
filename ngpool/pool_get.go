package ngpool

import (
	"bytes"
	"sort"

	"github.com/ngchain/ngcore/ngtypes"
)

// MaxTxsPerPack bounds how many txs one block template packs — the
// consensus cap, so a template never exceeds a valid block
const MaxTxsPerPack = ngtypes.MaxBlockTxCount

// GetPack returns a TxTrie of the txs packable at the height. The fee
// order (highest first, ties broken by the tx hash) decides WHICH txs
// survive the MaxTxsPerPack cap; the trie then re-sorts the survivors
// into the canonical in-block order (by tx hash)
func (pool *TxPool) GetPack(height uint64) ngtypes.TxTrie {
	pool.Lock()
	defer pool.Unlock()

	txs := make([]*ngtypes.FullTx, 0, len(pool.txMap))
	for _, tx := range pool.txMap {
		if tx != nil && tx.Height == height {
			txs = append(txs, tx)
		}
	}

	sort.Slice(txs, func(i, j int) bool {
		switch txs[i].Fee.Cmp(txs[j].Fee) {
		case 1:
			return true
		case -1:
			return false
		default:
			return bytes.Compare(txs[i].GetHash(), txs[j].GetHash()) < 0
		}
	})

	if len(txs) > MaxTxsPerPack {
		txs = txs[:MaxTxsPerPack]
	}

	return ngtypes.NewTxTrie(txs)
}

// GetCommitPack returns the commitments packable at the height, highest fee
// first (ties broken by the commitment hash). They ride the block's Commits
// list; the content root folds them in alongside the txs.
func (pool *TxPool) GetCommitPack(height uint64) []*ngtypes.Commitment {
	pool.Lock()
	defer pool.Unlock()

	commits := make([]*ngtypes.Commitment, 0, len(pool.commitMap))
	for _, c := range pool.commitMap {
		if c != nil && c.Height == height {
			commits = append(commits, c)
		}
	}

	sort.Slice(commits, func(i, j int) bool {
		switch commits[i].Fee.Cmp(commits[j].Fee) {
		case 1:
			return true
		case -1:
			return false
		default:
			return bytes.Compare(commits[i].Hash, commits[j].Hash) < 0
		}
	})

	if len(commits) > MaxTxsPerPack {
		commits = commits[:MaxTxsPerPack]
	}

	return commits
}
