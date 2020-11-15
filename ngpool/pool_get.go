package ngpool

import (
	"bytes"
	"sort"

	"github.com/ngchain/ngcore/ngtypes"
)

// GetPack will gives a sorted TxTire.
func (pool *TxPool) GetPack(prevBlockHash []byte) *ngtypes.TxTrie {
	txs := make([]*ngtypes.Tx, 0)
	accountNums := make([]uint64, 0)

	for num := range pool.txMap {
		accountNums = append(accountNums, num)
	}

	sort.Slice(accountNums, func(i, j int) bool { return accountNums[i] < accountNums[j] })

	for _, num := range accountNums {
		if pool.txMap[num] != nil && bytes.Equal(pool.txMap[num].PrevBlockHash, prevBlockHash) {
			txs = append(txs, pool.txMap[num])
		}
	}

	trie := ngtypes.NewTxTrie(txs)
	// trie.Sort()

	return trie
}

// GetPackTxs limits the sorted TxTire's txs to meet block txs requirements.
// func (p *TxPool) GetPackTxs() []*ngtypes.Tx {
// 	txs := p.GetPack().Txs
// 	size := 0

// 	for i := 0; i < len(txs); i++ {
// 		size += proto.Size(txs[i])
// 		if size > ngtypes.BlockMaxTxsSize {
// 			return txs[:i]
// 		}
// 	}

// 	return txs
// }
