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

	newBlockCh chan *ngtypes.Block

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

// Init inits the txPool with submodules.
func (p *TxPool) Init(newBlockCh chan *ngtypes.Block) {
	p.newBlockCh = newBlockCh
}

// Run starts listening to the new block & vault.
func (p *TxPool) Run() {
	go func() {
		for {
			block := <-p.newBlockCh
			log.Infof("start popping txs in block@%d", block.GetHeight())
			p.DelBlockTxs(block.Txs...)
		}
	}()
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
