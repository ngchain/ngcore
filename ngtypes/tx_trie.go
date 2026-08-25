package ngtypes

import (
	"bytes"
	"sort"

	"github.com/cbergoon/merkletree"
	"hash"

	"lukechampine.com/blake3"
)

// TxTrie is a fixed ordered tx container to get the trie root hash.
// This is not thread-safe
type TxTrie []*FullTx

// NewTxTrie receives ordered ops.
func NewTxTrie(txs []*FullTx) TxTrie {
	sort.Slice(txs, func(i, j int) bool {
		return bytes.Compare(txs[i].GetHash(), txs[j].GetHash()) < 0
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

// TrieRoot sort tx tire by trie tree and return the root hash. It commits
// only the txs; blocks fold in their commitments via ContentRoot.
func (tt *TxTrie) TrieRoot() []byte {
	if len(*tt) == 0 {
		return make([]byte, HashSize)
	}

	contents := make([]merkletree.Content, len(*tt))
	for i := range *tt {
		contents[i] = (*tt)[i]
	}

	return contentRoot(contents)
}

// contentRoot builds the merkle root over the given contents, deterministically
// ordered by their CalculateHash bytes (so txs and commitments interleave into
// one canonical order). An empty set is the zero hash.
func contentRoot(contents []merkletree.Content) []byte {
	if len(contents) == 0 {
		return make([]byte, HashSize)
	}

	sort.Slice(contents, func(i, j int) bool {
		hi, _ := contents[i].CalculateHash()
		hj, _ := contents[j].CalculateHash()
		return bytes.Compare(hi, hj) < 0
	})

	trie, err := merkletree.NewTreeWithHashStrategy(contents, func() hash.Hash { return blake3.New(32, nil) })
	if err != nil {
		log.Error(err)
	}

	return trie.MerkleRoot()
}

// ContentRoot returns the merkle root over BOTH a block's txs and its
// commitments, folded into one canonical (hash-sorted) content set. The
// existing TxTrieHash — already in the pow preimage — now binds commitments
// too, so no header/preimage change is needed.
func ContentRoot(txs []*FullTx, commits []*Commitment) []byte {
	if len(txs) == 0 && len(commits) == 0 {
		return make([]byte, HashSize)
	}

	contents := make([]merkletree.Content, 0, len(txs)+len(commits))
	for i := range txs {
		contents = append(contents, txs[i])
	}
	for i := range commits {
		contents = append(contents, commits[i])
	}

	return contentRoot(contents)
}
