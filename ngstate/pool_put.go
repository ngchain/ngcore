package ngstate

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ngchain/ngcore/ngtypes"
)

// PutNewTxFromLocal puts tx from local(rpc) into txpool.
func (p *TxPool) PutNewTxFromLocal(tx *ngtypes.Tx) (err error) {
	log.Debugf("putting new tx %x from rpc", tx.Hash())

	err = p.PutTx(tx)
	if err != nil {
		return err
	}

	return nil
}

// PutNewTxFromRemote puts tx from local(rpc) into txpool.
func (p *TxPool) PutNewTxFromRemote(tx *ngtypes.Tx) (err error) {
	log.Debugf("putting new tx %x from p2p", tx.Hash())

	err = p.PutTx(tx)
	if err != nil {
		return err
	}

	return nil
}

// PutTx puts txs from network(p2p) or RPC into txpool, should check error before putting.
// TODO: implement me
func (p *TxPool) PutTx(tx *ngtypes.Tx) error {
	p.Lock()
	defer p.Unlock()

	if err := GetActiveState().CheckTxs(tx); err != nil {
		return fmt.Errorf("malformed tx, rejected: %v", err)
	}

	if !bytes.Equal(tx.PrevBlockHash, GetActiveState().prevBlockHash) {
		return fmt.Errorf("tx %x does not belong to current State, found %x, require %x",
			tx.Hash(), tx.PrevBlockHash, GetActiveState().prevBlockHash)
	}

	if p.txMap[tx.Convener] == nil ||
		new(big.Int).SetBytes(p.txMap[tx.Convener].Fee).Cmp(new(big.Int).SetBytes(tx.Fee)) < 0 {
		p.txMap[tx.Convener] = tx
	}

	return nil
}
