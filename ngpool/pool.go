package ngpool

import (
	"bytes"
	"sync"

	logging "github.com/ngchain/zap-log"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("ngpool")

// DefaultPoolSize bounds how many conveners can queue a tx at once
const DefaultPoolSize = 4096

// TxPool is a little mem db which stores **signed** tx.
// RULE: One Account can only send one Tx (a higher fee replaces),
// txs are height-locked to the next block, and every tip movement
// deprecates the whole pool.
type TxPool struct {
	sync.Mutex

	db    *bbolt.DB
	txMap map[ngtypes.Address]*ngtypes.FullTx // From address -> queued tx

	// MaxSize caps the pool; when full, a new tx must outbid the
	// cheapest queued one
	MaxSize int

	chain     *blockchain.Chain
	localNode *ngp2p.LocalNode
}

func Init(db *bbolt.DB, chain *blockchain.Chain, localNode *ngp2p.LocalNode) *TxPool {
	pool := &TxPool{
		Mutex: sync.Mutex{},
		db:    db,
		txMap: make(map[ngtypes.Address]*ngtypes.FullTx),

		MaxSize: DefaultPoolSize,

		chain:     chain,
		localNode: localNode,
	}

	return pool
}

// IsInPool checks one tx is in pool or not.
func (pool *TxPool) IsInPool(txHash []byte) (exists bool, inPoolTx *ngtypes.FullTx) {
	pool.Lock()
	defer pool.Unlock()

	for _, txInQueue := range pool.txMap {
		if bytes.Equal(txInQueue.GetHash(), txHash) {
			return true, txInQueue
		}
	}

	return false, nil
}

// Reset cleans all txs inside the pool.
func (pool *TxPool) Reset() {
	pool.Lock()
	defer pool.Unlock()

	pool.txMap = make(map[ngtypes.Address]*ngtypes.FullTx)
}
