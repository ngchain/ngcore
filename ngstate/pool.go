package ngstate

import (
	"bytes"
	"sync"

	"github.com/ngchain/ngcore/ngtypes"
)

// TxPool is a little mem db which stores **signed** tx.
// RULE: One Account can only send one Tx, all Txs will be accepted
// Every time the state updated, the old pool will be deprecated
type TxPool struct {
	sync.Mutex
	txMap map[uint64]*ngtypes.Tx // priority first
}

func NewTxPool() *TxPool {
	return &TxPool{
		txMap: make(map[uint64]*ngtypes.Tx, 0),
	}
}

// IsInPool checks one tx is in pool or not. TODO: export it into rpc.
func (p *TxPool) IsInPool(txHash []byte) (exists bool, inPoolTx *ngtypes.Tx) {
	for _, txInQueue := range p.txMap {
		if bytes.Equal(txInQueue.Hash(), txHash) {
			return true, txInQueue
		}
	}

	return false, nil
}
