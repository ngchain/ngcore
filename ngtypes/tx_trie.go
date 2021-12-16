package ngtypes

import (
	"sort"

	"github.com/cbergoon/merkletree"
	"golang.org/x/crypto/sha3"
)

// TxTrie is a fixed ordered tx container to get the trie root hash.
// This is not thread-safe
type TxTrie []*FullTx

// NewTxTrie receives ordered ops.
func NewTxTrie(txs []*FullTx) TxTrie {
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].Convener < txs[j].Convener
	})
	return txs
}

// func (tt *TxTrie) Len() int {
// 	return len(tt.Txs)
// }

// Less means that the tx (I) has lower priority (than J).
// func (tt *TxTrie) Less(i, j int) bool {
// 	return new(big.Int).SetBytes(tt.Txs[i].Fee).Cmp(new(big.Int).SetBytes(tt.Txs[j].Fee)) < 0 ||
// 		tt.Txs[i].Convener > tt.Txs[j].Convener
// }

// Swap just swap the values of txs.
// func (tt *TxTrie) Swap(i, j int) {
// 	tt.Txs[i], tt.Txs[j] = tt.Txs[j], tt.Txs[i]
// }

// Sort will sort the txs from lower priority to higher priority.
// func (tt *TxTrie) Sort() *TxTrie {
// 	sort.Sort(tt)
// 	return tt
// }

// ReverseSort will sort the txs from higher priority to lower priority.
// func (tt *TxTrie) ReverseSort() *TxTrie {
// 	return sort.Reverse(tt).(*TxTrie)
// }

// Contains determine if tt.Txs and tx are equal.
func (tt *TxTrie) Contains(tx *FullTx) bool {
	for i := 0; i < len(*tt); i++ {
		if (*tt)[i] == tx {
			return true
		}
	}

	return false
}

// TrieRoot sort tx tire by trie tree and return the root hash.
func (tt *TxTrie) TrieRoot() []byte {
	if len(*tt) == 0 {
		return make([]byte, HashSize)
	}

	mtc := make([]merkletree.Content, len(*tt))
	for i := range *tt {
		mtc[i] = (*tt)[i]
	}

	trie, err := merkletree.NewTreeWithHashStrategy(mtc, sha3.New256)
	if err != nil {
		log.Error(err)
	}

	return trie.MerkleRoot()
}
