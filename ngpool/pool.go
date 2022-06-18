package ngpool

import (
	"bytes"
	"sync"

	"github.com/c0mm4nd/dbolt"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("ngpool")

// TxPool is a little mem db which stores **signed** tx.
// RULE: One Account can only send one Tx, all Txs will be accepted
// Every time the state updated, the old pool will be deprecated.
type TxPool struct {
	sync.Mutex

	db    *dbolt.DB
	txMap map[uint64]*ngtypes.FullTx // priority first

	chain     *blockchain.Chain
	localNode *ngp2p.LocalNode
}

func Init(db *dbolt.DB, chain *blockchain.Chain, localNode *ngp2p.LocalNode) *TxPool {
	pool := &TxPool{
		Mutex: sync.Mutex{},
		db:    db,
		txMap: make(map[uint64]*ngtypes.FullTx),

		chain:     chain,
		localNode: localNode,
	}

	return pool
}

// IsInPool checks one tx is in pool or not.
func (pool *TxPool) IsInPool(txHash []byte) (exists bool, inPoolTx *ngtypes.FullTx) {
	for _, txInQueue := range pool.txMap {
		if bytes.Equal(txInQueue.GetHash(), txHash) {
			return true, txInQueue
		}
	}

	return false, nil
}

// Reset cleans all txs inside the pool.
func (pool *TxPool) Reset() {
	pool.txMap = make(map[uint64]*ngtypes.FullTx)
}
