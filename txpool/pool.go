package txpool

import (
	"sync"

	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("txpool")

// TxPool is a little mem db which stores **signed** tx
type TxPool struct {
	sync.RWMutex

	sheetManager *ngsheet.SheetManager

	Queuing map[uint64]map[uint64]*ngtypes.Tx // map[accountID] map[nonce]Tx

	newBlockCh chan *ngtypes.Block

	NewCreatedTxEvent chan *ngtypes.Tx
}

var txpool *TxPool

func NewTxPool(sheetManager *ngsheet.SheetManager) *TxPool {
	if txpool == nil {
		txpool = &TxPool{
			sheetManager: sheetManager,

			Queuing: make(map[uint64]map[uint64]*ngtypes.Tx),

			NewCreatedTxEvent: make(chan *ngtypes.Tx),
		}
	}

	return txpool
}

func GetTxPool() *TxPool {
	if txpool == nil {
		panic("txpool is closed")
	}

	return txpool
}

// Init inits the txPool with submodules
func (p *TxPool) Init(newBlockCh chan *ngtypes.Block) {
	p.newBlockCh = newBlockCh
}

// Run starts listening to the new block & vault
func (p *TxPool) Run() {
	go func() {
		for {
			select {
			case block := <-p.newBlockCh:
				log.Infof("start popping txs in block@%d", block.GetHeight())
				p.DelBlockTxs(block.Txs...)
			}
		}
	}()
}

// IsInPool checks one tx is in pool or not
// TODO: export it into rpc
func (p *TxPool) IsInPool(tx *ngtypes.Tx) (exists bool) {
	_, exists = p.Queuing[tx.GetConvener()]
	if !exists {
		return
	}

	exists = p.Queuing[tx.GetConvener()][tx.GetNonce()] != nil

	return
}
