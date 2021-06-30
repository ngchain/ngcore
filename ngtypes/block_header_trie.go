package ngtypes

import (
	"github.com/cbergoon/merkletree"
	"golang.org/x/crypto/sha3"
	"math/big"
	"sort"
)

// HeaderTrie is a fixed ordered block header container of the subBlocks
type HeaderTrie []*BlockHeader

// NewHeaderTrie creates new HeaderTrie
func NewHeaderTrie(headers []*BlockHeader) HeaderTrie {
	sort.Slice(headers, func(i, j int) bool {
		// i nonce < j nonce
		return new(big.Int).SetBytes(headers[i].Nonce).Cmp(new(big.Int).SetBytes(headers[j].Nonce)) < 0
	})
	return headers
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
func (ht *HeaderTrie) Contains(h *BlockHeader) bool {
	for i := 0; i < len(*ht); i++ {
		if (*ht)[i] == h {
			return true
		}
	}

	return false
}

// TrieRoot sort tx tire by trie tree and return the root hash.
func (ht *HeaderTrie) TrieRoot() []byte {
	if len(*ht) == 0 {
		return make([]byte, HashSize)
	}

	mtc := make([]merkletree.Content, len(*ht))
	for i := range *ht {
		mtc[i] = (*ht)[i]
	}

	trie, err := merkletree.NewTreeWithHashStrategy(mtc, sha3.New256)
	if err != nil {
		log.Error(err)
	}

	return trie.MerkleRoot()
}
