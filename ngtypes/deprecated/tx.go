package deprecated

//import (
//	"errors"
//	"github.com/cbergoon/merkletree"
//	"math/big"
//	"sort"
//)
//
//// TxTrie is an fixed ordered operation container, mainly for pending
//// And TxTrie is an advanced type, aiming to get the trie root hash
//type TxTrie struct {
//	Txs []*Transaction
//}
//
//// NewTxTrie receives ordered ops
//func NewTxTrie(txs []*Transaction) *TxTrie {
//	return &TxTrie{
//		Txs: txs,
//	}
//}
//
//func (tt *TxTrie) Len() int {
//	return len(tt.Txs)
//}
//
//// Less means that the op (I) has lower priority (than J)
//func (tt *TxTrie) Less(i, j int) bool {
//	return new(big.Int).SetBytes(tt.Txs[i].Fee).Cmp(new(big.Int).SetBytes(tt.Txs[j].Fee)) < 0 || tt.Txs[i].Convener > tt.Txs[j].Convener
//}
//
//func (tt *TxTrie) Swap(i, j int) {
//	tmp := tt.Txs[i]
//	tt.Txs[i] = tt.Txs[j]
//	tt.Txs[j] = tmp
//}
//
//// Sort the ops from lower priority to higher priority
//func (tt *TxTrie) Sort() *TxTrie {
//	sort.Sort(tt)
//	return tt
//}
//
//// ReverseSort the ops from higher priority to lower priority
//func (tt *TxTrie) ReverseSort() *TxTrie {
//	return sort.Reverse(tt).(*TxTrie)
//}
//
//func (tt *TxTrie) Append(tx *Transaction) {
//	tt.Txs = append(tt.Txs, tx)
//}
//
//func (tt *TxTrie) Del(tx *Transaction) error {
//	for i := range tt.Txs {
//		if tt.Txs[i] == tx {
//			tt.Txs = append(tt.Txs[:i], tt.Txs[i+1:]...)
//			return nil
//		}
//	}
//
//	return errors.New("no such operation")
//}
//
//func (tt *TxTrie) Contains(tx *Transaction) bool {
//	for i := 0; i < len(tt.Txs); i++ {
//		if tt.Txs[i] == tx {
//			return true
//		}
//	}
//	return false
//}
//
//func (tt *TxTrie) TrieRoot() []byte {
//	if len(tt.Txs) == 0 {
//		return make([]byte, 32)
//	}
//
//	trie, err := merkletree.NewTree(TxsToMerkleTreeContents(tt.Txs))
//	if err != nil {
//		log.Error(err)
//	}
//
//	return trie.MerkleRoot()
//}
//
//// TxBucket is an transaction container with unfixed order, mainly for implementing queuing
//type TxBucket struct {
//	Txs map[uint64]map[uint64]*Transaction// key1=Convener key2=N
//}
//
//func NewTxBucket() *TxBucket {
//	return &TxBucket{
//		Txs: make(map[uint64]map[uint64]*Transaction, 0),
//	}
//}
//
//func (tb *TxBucket) Put(tx *Transaction) {
//	if tb.Txs[tx.Convener] == nil {
//		tb.Txs[tx.Convener] = map[uint64]*Transaction{
//			tx.Nonce: tx,
//		}
//		return
//	}
//
//	tb.Txs[tx.Convener][tx.Nonce] = tx
//	return
//}
//
//func (tb *TxBucket) Del(tx *Transaction) error {
//	if tb.Txs[tx.Convener] == nil {
//		return errors.New("no such operation")
//	}
//
//	if tb.Txs[tx.Convener][tx.Nonce] == nil {
//		return errors.New("no such operation")
//	}
//
//	tb.Txs[tx.Convener][tx.Nonce] = nil
//	return nil
//}
//
//// Get will return the tx by convener and nonce, but if no such tx, it will return nil
//func (tb *TxBucket) Get(convener uint64, nonce uint64) *Transaction {
//	_, exists := tb.Txs[convener]
//	if !exists {
//		return nil
//	}
//
//	return tb.Txs[convener][nonce]
//}
//
//func (tb *TxBucket) Has(tx *Transaction) bool {
//	// avoid panic when the convener is missing
//	_, exists := tb.Txs[tx.Convener]
//	if !exists {
//		return false
//	}
//
//	// locate the tx
//	_, exists = tb.Txs[tx.Convener][tx.Nonce]
//	return exists
//}
//
//func (tb *TxBucket) GetSortedTrie() *TxTrie {
//	var slice []*Transaction
//	for i := range tb.Txs {
//		if tb.Txs[i] != nil && len(tb.Txs[i]) > 0 {
//			for j := range tb.Txs[i] {
//				slice = append(slice, tb.Txs[i][j])
//			}
//		}
//	}
//	trie := NewTxTrie(slice)
//	trie.Sort()
//	return trie
//}
//
