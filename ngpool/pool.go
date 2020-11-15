package ngpool

import (
	"bytes"
	"sync"

	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("ngpool")

// TxPool is a little mem db which stores **signed** tx.
// RULE: One Account can only send one Tx, all Txs will be accepted
// Every time the state updated, the old pool will be deprecated
type TxPool struct {
	sync.Mutex

	db    *badger.DB
	txMap map[uint64]*ngtypes.Tx // priority first

	chain     *ngchain.Chain
	localNode *ngp2p.LocalNode
}

func Init(db *badger.DB, chain *ngchain.Chain, localNode *ngp2p.LocalNode) *TxPool {
	pool := &TxPool{
		Mutex: sync.Mutex{},
		db:    db,
		txMap: make(map[uint64]*ngtypes.Tx),

		chain:     chain,
		localNode: localNode,
	}

	return pool
}

// IsInPool checks one tx is in pool or not.
func (pool *TxPool) IsInPool(txHash []byte) (exists bool, inPoolTx *ngtypes.Tx) {
	for _, txInQueue := range pool.txMap {
		if bytes.Equal(txInQueue.Hash(), txHash) {
			return true, txInQueue
		}
	}

	return false, nil
}

// IsInPool checks one tx is in pool or not.
func (pool *TxPool) Reset() {
	pool.txMap = make(map[uint64]*ngtypes.Tx)
}
