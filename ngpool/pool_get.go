package ngpool

import (
	"sort"

	"github.com/ngchain/ngcore/ngtypes"
)

// MaxTxsPerPack bounds how many txs one block template packs
const MaxTxsPerPack = 512

// GetPack returns a TxTrie of the txs packable at the height. The fee
// order (highest first, ties broken by the lower convener num) decides
// WHICH txs survive the MaxTxsPerPack cap; the trie then re-sorts the
// survivors into the canonical in-block order (by convener)
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
			return txs[i].Convener < txs[j].Convener
		}
	})

	if len(txs) > MaxTxsPerPack {
		txs = txs[:MaxTxsPerPack]
	}

	return ngtypes.NewTxTrie(txs)
}
