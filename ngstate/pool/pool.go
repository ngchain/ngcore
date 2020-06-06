package pool

import (
	"github.com/ngchain/ngcore/ngtypes"
)

// TxPool is a little mem db which stores **signed** tx.
// TODO: !important embed txpool into ngstate!
type TxPool struct {
	Queuing []*ngtypes.Tx // priority first

	NewCreatedTxEvent chan *ngtypes.Tx
}

var txpool *TxPool

// init will create a new global txpool.
func init() {
	txpool = &TxPool{
		Queuing: make([]*ngtypes.Tx, 0),

		NewCreatedTxEvent: make(chan *ngtypes.Tx),
	}
}

// GetTxPool will return the registered global txpool.
func GetTxPool() *TxPool {
	if txpool == nil {
		panic("txpool is not initialized")
	}

	return txpool
}

// HandleNewBlock will help txpool delete the txs in block
func (p *TxPool) HandleNewBlock(block *ngtypes.Block) {
	log.Infof("start popping txs in block@%d", block.GetHeight())
	p.DelBlockTxs(block.Txs...)
}

// IsInPool checks one tx is in pool or not. TODO: export it into rpc.
func (p *TxPool) IsInPool(tx *ngtypes.Tx) (exists bool) {
	_, exists = p.Queuing[tx.Header.GetConvener()]
	if !exists {
		return
	}

	exists = p.Queuing[tx.Header.GetConvener()][tx.Header.GetN()] != nil

	return
}
