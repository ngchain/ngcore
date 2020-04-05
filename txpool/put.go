package txpool

import (
	"errors"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
)

// PutNewTxFromLocal puts tx from local(rpc) into txpool
func (p *TxPool) PutNewTxFromLocal(tx *ngtypes.Transaction) (err error) {
	err = p.PutTxs(tx)
	if err != nil {
		return err
	}

	if err = p.CheckTxs(tx); err != nil {
		return fmt.Errorf("malformed tx, rejected: %v", err)
	}

	p.NewCreatedTxEvent <- tx

	return nil
}

// PutTxs puts txs from network(p2p) into txpool
func (p *TxPool) PutTxs(txs ...*ngtypes.Transaction) error {
	p.Lock()
	defer p.Unlock()

	if err := p.CheckTxs(txs...); err != nil {
		return fmt.Errorf("malformed tx in txs, reject all txs: %v", err)
	}

	var err error
	for i := range txs {
		if !txs[i].IsSigned() {
			err = errors.New("cannot putting unsigned tx, " + txs[i].HashHex() + " into queuing")
			log.Error(err)

			continue
		}

		if n := p.CurrentVault.Sheet.Accounts[txs[i].GetConvener()].Nonce + 1; txs[i].GetNonce() != n+1 {
			err = errors.New("Tx" + txs[i].HashHex() + "'s nonce is incorrect")
			log.Error(err)

			continue
		}

		p.Queuing[txs[i].GetConvener()][txs[i].GetNonce()] = txs[i]
	}

	return err
}
