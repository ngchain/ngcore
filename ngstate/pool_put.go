package ngstate

import (
	"bytes"
	"fmt"

	"github.com/ngchain/ngcore/ngtypes"
)

// PutNewTxFromLocal puts tx from local(rpc) into txpool.
func (p *TxPool) PutNewTxFromLocal(tx *ngtypes.Tx) (err error) {
	log.Debugf("putting new tx %s from rpc", tx.BS58())

	err = p.PutTxs(tx)
	if err != nil {
		return err
	}

	return nil
}

// PutNewTxFromRemote puts tx from local(rpc) into txpool.
func (p *TxPool) PutNewTxFromRemote(tx *ngtypes.Tx) (err error) {
	log.Debugf("putting new tx %s from p2p", tx.BS58())

	err = p.PutTxs(tx)
	if err != nil {
		return err
	}

	return nil
}

// PutTxs puts txs from network(p2p) into txpool, should check error before putting.
// TODO: implement me
func (p *TxPool) PutTxs(txs ...*ngtypes.Tx) error {
	p.Lock()
	defer p.Unlock()

	if err := GetCurrentState().CheckTxs(txs...); err != nil {
		return fmt.Errorf("malformed tx, rejected: %v", err)
	}

	for _, tx := range txs {
		if !bytes.Equal(tx.PrevBlockHash, GetCurrentState().prevSheetkHash) {
			return fmt.Errorf("tx %s does not belong to current State", tx.Hash())
		}
	}

	p.txs = append(p.txs, txs...)

	return nil
}
