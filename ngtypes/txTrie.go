package ngtypes

import (
	"errors"
	"math/big"
	"sort"

	"github.com/cbergoon/merkletree"
)

// TxTrie is an fixed ordered operation container, mainly for pending
// And TxTrie is an advanced type, aiming to get the trie root hash
type TxTrie struct {
	Txs []*Transaction
}

// NewTxTrie receives ordered ops
func NewTxTrie(txs []*Transaction) *TxTrie {
	return &TxTrie{
		Txs: txs,
	}
}

func (tt *TxTrie) Len() int {
	return len(tt.Txs)
}

// Less means that the op (I) has lower priority (than J)
func (tt *TxTrie) Less(i, j int) bool {
	return new(big.Int).SetBytes(tt.Txs[i].Header.Fee).Cmp(new(big.Int).SetBytes(tt.Txs[j].Header.Fee)) < 0 || tt.Txs[i].Header.Convener > tt.Txs[j].Header.Convener
}

func (tt *TxTrie) Swap(i, j int) {
	tmp := tt.Txs[i]
	tt.Txs[i] = tt.Txs[j]
	tt.Txs[j] = tmp
}

// Sort the ops from lower priority to higher priority
func (tt *TxTrie) Sort() *TxTrie {
	sort.Sort(tt)
	return tt
}

// ReverseSort the ops from higher priority to lower priority
func (tt *TxTrie) ReverseSort() *TxTrie {
	return sort.Reverse(tt).(*TxTrie)
}

func (tt *TxTrie) Append(tx *Transaction) {
	tt.Txs = append(tt.Txs, tx)
}

func (tt *TxTrie) Del(tx *Transaction) error {
	for i := range tt.Txs {
		if tt.Txs[i] == tx {
			tt.Txs = append(tt.Txs[:i], tt.Txs[i+1:]...)
			return nil
		}
	}

	return errors.New("no such operation")
}

func (tt *TxTrie) Contains(tx *Transaction) bool {
	for i := 0; i < len(tt.Txs); i++ {
		if tt.Txs[i] == tx {
			return true
		}
	}
	return false
}

func (tt *TxTrie) TrieRoot() []byte {
	if len(tt.Txs) == 0 {
		return make([]byte, 32)
	}

	trie, err := merkletree.NewTree(TxsToMerkleTreeContents(tt.Txs))
	if err != nil {
		log.Error(err)
	}

	return trie.MerkleRoot()
}
