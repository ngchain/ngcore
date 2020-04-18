package txpool

import (
	"github.com/ngchain/ngcore/ngtypes"
)

// DelBlockTxs will popping txs from txpool
func (p *TxPool) DelBlockTxs(txs ...*ngtypes.Tx) {
	p.Lock()
	defer p.Unlock()

	for i := range txs {
		if p.Queuing[txs[i].GetConvener()] != nil {
			delete(p.Queuing[txs[i].GetConvener()], txs[i].GetNonce())
		}
	}

	return
}
