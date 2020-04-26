package ngtypes

import (
	"errors"
	"math/big"
	"sort"

	"github.com/cbergoon/merkletree"
	"golang.org/x/crypto/sha3"
)

// TxTrie is a fixed ordered tx container, mainly for pending.
// And TxTrie is an advanced type, aiming to get the trie root hash.
type TxTrie struct {
	Txs []*Tx
}

// NewTxTrie receives ordered ops.
func NewTxTrie(txs []*Tx) *TxTrie {
	return &TxTrie{
		Txs: txs,
	}
}

func (tt *TxTrie) Len() int {
	return len(tt.Txs)
}

// Less means that the tx (I) has lower priority (than J).
func (tt *TxTrie) Less(i, j int) bool {
	return new(big.Int).SetBytes(tt.Txs[i].Header.Fee).Cmp(new(big.Int).SetBytes(tt.Txs[j].Header.Fee)) < 0 ||
		tt.Txs[i].Header.Convener > tt.Txs[j].Header.Convener
}

// Swap just swap the values of txs.
func (tt *TxTrie) Swap(i, j int) {
	tt.Txs[i], tt.Txs[j] = tt.Txs[j], tt.Txs[i]
}

// Sort will sort the txs from lower priority to higher priority.
func (tt *TxTrie) Sort() *TxTrie {
	sort.Sort(tt)
	return tt
}

// ReverseSort will sort the txs from higher priority to lower priority.
func (tt *TxTrie) ReverseSort() *TxTrie {
	return sort.Reverse(tt).(*TxTrie)
}

// Append will append new tx to the end of TxTrie's txs.
func (tt *TxTrie) Append(tx *Tx) {
	tt.Txs = append(tt.Txs, tx)
}

// Del removes a tx from txs.
func (tt *TxTrie) Del(tx *Tx) error {
	for i := range tt.Txs {
		if tt.Txs[i] == tx {
			tt.Txs = append(tt.Txs[:i], tt.Txs[i+1:]...)
			return nil
		}
	}

	return errors.New("no such transaction")
}

// Contains determine if tt.Txs and tx are equal.
func (tt *TxTrie) Contains(tx *Tx) bool {
	for i := 0; i < len(tt.Txs); i++ {
		if tt.Txs[i] == tx {
			return true
		}
	}

	return false
}

// TrieRoot sort tx tire by trie tree and return the root hash.
func (tt *TxTrie) TrieRoot() []byte {
	if len(tt.Txs) == 0 {
		return make([]byte, 32)
	}

	trie, err := merkletree.NewTreeWithHashStrategy(TxsToMerkleTreeContents(tt.Txs), sha3.New256)
	if err != nil {
		log.Error(err)
	}

	return trie.MerkleRoot()
}
