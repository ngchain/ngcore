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

	state *State
	txMap map[uint64]*ngtypes.Tx // priority first
}

// GetTxPool will return the registered global txpool.
func GetTxPool() *TxPool {
	if manager == nil {
		panic("state manager is not initialized")
	}

	if manager.currentState == nil {
		panic("state is not initialized")
	}

	if manager.currentState.pool == nil {
		panic("txpool is not initialized")
	}

	return manager.currentState.pool
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
