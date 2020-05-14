package txpool

import (
	"sync"

	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("txpool")

// TxPool is a little mem db which stores **signed** tx.
type TxPool struct {
	sync.RWMutex

	status *ngstate.State

	Queuing map[uint64]map[uint64]*ngtypes.Tx // map[accountID] map[nonce]Tx

	NewCreatedTxEvent chan *ngtypes.Tx
}

var txpool *TxPool

// NewTxPool will create a new global txpool.
func NewTxPool(status *ngstate.State) *TxPool {
	if txpool == nil {
		txpool = &TxPool{
			status: status,

			Queuing: make(map[uint64]map[uint64]*ngtypes.Tx),

			NewCreatedTxEvent: make(chan *ngtypes.Tx),
		}
	}

	return txpool
}

// GetTxPool will return the registered global txpool.
func GetTxPool() *TxPool {
	if txpool == nil {
		panic("txpool is closed")
	}

	return txpool
}

func (p *TxPool) HandleNewBlock(block *ngtypes.Block) {
	log.Infof("start popping txs in block@%d", block.GetHeight())
	p.DelBlockTxs(block.Txs...)
}

// IsInPool checks one tx is in pool or not. TODO: export it into rpc.
func (p *TxPool) IsInPool(tx *ngtypes.Tx) (exists bool) {
	_, exists = p.Queuing[tx.GetConvener()]
	if !exists {
		return
	}

	exists = p.Queuing[tx.GetConvener()][tx.GetN()] != nil

	return
}
