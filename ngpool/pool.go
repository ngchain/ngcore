package ngpool

import (
	"bytes"
	"github.com/dgraph-io/badger/v2"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"sync"
)

var log = logging.Logger("ngpool")

// TxPool is a little mem db which stores **signed** tx.
// RULE: One Account can only send one Tx, all Txs will be accepted
// Every time the state updated, the old pool will be deprecated
type TxPool struct {
	sync.Mutex

	db    *badger.DB
	txMap map[uint64]*ngtypes.Tx // priority first
}

var pool *TxPool

func Init(db *badger.DB) {
	pool = &TxPool{
		db:    db,
		txMap: make(map[uint64]*ngtypes.Tx),
	}
}

// IsInPool checks one tx is in pool or not. TODO: export it into rpc.
func IsInPool(txHash []byte) (exists bool, inPoolTx *ngtypes.Tx) {
	for _, txInQueue := range pool.txMap {
		if bytes.Equal(txInQueue.Hash(), txHash) {
			return true, txInQueue
		}
	}

	return false, nil
}
