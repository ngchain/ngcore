package pool

import (
	"github.com/golang/protobuf/proto"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/ngcore/ngtypes"
	"sync"
)

var log = logging.Logger("pool")

// TxPool is a little mem db which stores **signed** tx.
// TODO: !important embed txpool into ngstate!
type TxPool struct {
	sync.RWMutex

	currentPrevBlockHash []byte
	Queuing              []*ngtypes.Tx // priority first
}

var txpool *TxPool

// init will create a new global txpool.
func init() {
	txpool = &TxPool{
		currentPrevBlockHash: ngtypes.GenesisBlockHash,
		Queuing:              make([]*ngtypes.Tx, 0),
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
	for _, txInQueue := range p.Queuing {
		if proto.Equal(tx, txInQueue) {
			return true
		}
	}

	return
}

func (p *TxPool) OnNewBlock(blockHash []byte) {
	p.Lock()
	defer p.Unlock()

	p.currentPrevBlockHash = blockHash
	p.Queuing = make([]*ngtypes.Tx, 0)

}
