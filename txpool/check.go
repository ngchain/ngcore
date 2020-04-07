package txpool

import (
	"github.com/ngchain/ngcore/ngtypes"
)

func (p *TxPool) DelBlockTxs(txs ...*ngtypes.Tx) {
	p.Lock()
	defer p.Unlock()

	for i := range txs {
		if p.Queuing[txs[i].GetConvener()] != nil {
			delete(p.Queuing[txs[i].GetConvener()], txs[i].GetNonce())
		}
	}
}

// CheckTxs will check txs self and error **in sheet**
func (p *TxPool) CheckTxs(txs ...*ngtypes.Tx) error {
	return p.sheetManager.CheckTxs(txs...)
}
