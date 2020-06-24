package ngstate

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
)

// PutNewTxFromLocal puts tx from local(rpc) into txpool.
func (p *TxPool) PutNewTxFromLocal(tx *ngtypes.Tx) (err error) {
	log.Debugf("putting new tx %s from rpc", tx.BS58())

	err = p.PutTx(tx)
	if err != nil {
		return err
	}

	return nil
}

// PutNewTxFromRemote puts tx from local(rpc) into txpool.
func (p *TxPool) PutNewTxFromRemote(tx *ngtypes.Tx) (err error) {
	log.Debugf("putting new tx %s from p2p", tx.BS58())

	err = p.PutTx(tx)
	if err != nil {
		return err
	}

	return nil
}

// PutTxs puts txs from network(p2p) into txpool, should check error before putting.
// TODO: implement me
func (p *TxPool) PutTx(tx *ngtypes.Tx) error {
	p.Lock()
	defer p.Unlock()

	if err := GetCurrentState().CheckTxs(tx); err != nil {
		return fmt.Errorf("malformed tx, rejected: %v", err)
	}

	if !bytes.Equal(tx.PrevBlockHash, GetCurrentState().prevSheetHash) {
		return fmt.Errorf("tx %s does not belong to current State", tx.Hash())
	}

	if p.txMap[tx.Convener] == nil ||
		new(big.Int).SetBytes(p.txMap[tx.Convener].Fee).Cmp(new(big.Int).SetBytes(tx.Fee)) < 0 {
		p.txMap[tx.Convener] = tx
	}

	return nil
}
