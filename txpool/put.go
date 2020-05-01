package txpool

import (
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
)

// PutNewTxFromLocal puts tx from local(rpc) into txpool.
func (p *TxPool) PutNewTxFromLocal(tx *ngtypes.Tx) (err error) {
	log.Debugf("putting new tx %s from rpc", tx.BS58())

	if err = p.sheetManager.CheckTxs(tx); err != nil {
		return fmt.Errorf("malformed tx, rejected: %v", err)
	}

	err = p.PutTxs(tx)
	if err != nil {
		return err
	}

	p.NewCreatedTxEvent <- tx

	return nil
}

// PutTxs puts txs from network(p2p) into txpool, should check error before putting.
func (p *TxPool) PutTxs(txs ...*ngtypes.Tx) error {
	p.Lock()
	defer p.Unlock()

	for i := range txs {
		if p.Queuing[txs[i].GetConvener()] == nil {
			p.Queuing[txs[i].GetConvener()] = make(map[uint64]*ngtypes.Tx)
		}

		p.Queuing[txs[i].GetConvener()][txs[i].GetN()] = txs[i]
	}

	return nil
}
