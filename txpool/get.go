package txpool

import (
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/ngcore/ngtypes"
)

// GetPack will gives a sorted TxTire
func (p *TxPool) GetPack() *ngtypes.TxTrie {
	var txs []*ngtypes.Tx
	for i := range p.Queuing {
		if p.Queuing[i] != nil && len(p.Queuing[i]) > 0 {
			for j := range p.Queuing[i] {
				txs = append(txs, p.Queuing[i][j])
			}
		}
	}
	trie := ngtypes.NewTxTrie(txs)
	trie.Sort()
	return trie
}

// GetPackTxs limits the sorted TxTire's txs to meet block txs requirements
func (p *TxPool) GetPackTxs() []*ngtypes.Tx {
	txs := p.GetPack().Txs
	size := 0

	for i := 0; i < len(txs); i++ {
		size += proto.Size(txs[i])
		if size > ngtypes.BlockMaxTxsSize {
			return txs[:i]
		}
	}

	return txs
}
