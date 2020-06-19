package ngstate

import (
	"sync"

	"github.com/golang/protobuf/proto"
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

	if manager.CurrentState == nil {
		panic("state is not initialized")
	}

	if manager.CurrentState.pool == nil {
		panic("txpool is not initialized")
	}

	return manager.CurrentState.pool
}

// IsInPool checks one tx is in pool or not. TODO: export it into rpc.
func (p *TxPool) IsInPool(tx *ngtypes.Tx) (exists bool) {
	for _, txInQueue := range p.txMap {
		if proto.Equal(tx, txInQueue) {
			return true
		}
	}

	return
}
