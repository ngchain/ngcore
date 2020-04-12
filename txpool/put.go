package txpool

import (
	"errors"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
)

// PutNewTxFromLocal puts tx from local(rpc) into txpool
func (p *TxPool) PutNewTxFromLocal(tx *ngtypes.Tx) (err error) {
	err = p.PutTxs(tx)
	if err != nil {
		return err
	}

	if err = p.sheetManager.CheckCurrentTxs(tx); err != nil {
		return fmt.Errorf("malformed tx, rejected: %v", err)
	}

	p.NewCreatedTxEvent <- tx

	return nil
}

// PutTxs puts txs from network(p2p) into txpool
func (p *TxPool) PutTxs(txs ...*ngtypes.Tx) error {
	p.Lock()
	defer p.Unlock()

	if err := p.sheetManager.CheckCurrentTxs(txs...); err != nil {
		return fmt.Errorf("malformed tx in txs, reject all txs: %v", err)
	}

	var err error
	for i := range txs {
		if !txs[i].IsSigned() {
			err = errors.New("cannot putting unsigned tx, " + txs[i].BS58() + " into queuing")
			log.Error(err)

			continue
		}

		if p.Queuing[txs[i].GetConvener()] == nil {
			p.Queuing[txs[i].GetConvener()] = make(map[uint64]*ngtypes.Tx)
		}
		p.Queuing[txs[i].GetConvener()][txs[i].GetNonce()] = txs[i]
	}

	return err
}
