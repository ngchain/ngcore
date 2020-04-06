package txpool

import (
	"sync"

	"github.com/whyrusleeping/go-logging"

	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.MustGetLogger("txpool")

// TxPool is a little mem db which stores **signed** tx
type TxPool struct {
	sync.RWMutex

	sheetManager *ngsheet.Manager

	Queuing      map[uint64]map[uint64]*ngtypes.Tx // map[accountID] map[nonce]Tx
	CurrentVault *ngtypes.Vault

	newBlockCh chan *ngtypes.Block
	newVaultCh chan *ngtypes.Vault

	NewCreatedTxEvent chan *ngtypes.Tx
}

func NewTxPool(sheetManager *ngsheet.Manager) *TxPool {
	return &TxPool{
		sheetManager: sheetManager,

		Queuing:      make(map[uint64]map[uint64]*ngtypes.Tx),
		CurrentVault: nil,

		NewCreatedTxEvent: make(chan *ngtypes.Tx),
	}
}

// Init inits the txPool with submodules
func (p *TxPool) Init(currentVault *ngtypes.Vault, newBlockCh chan *ngtypes.Block, newVaultCh chan *ngtypes.Vault) {
	p.CurrentVault = currentVault
	p.newBlockCh = newBlockCh
	p.newVaultCh = newVaultCh
}

// Run starts listening to the new block & vault
func (p *TxPool) Run() {
	go func() {
		for {
			select {
			case block := <-p.newBlockCh:
				log.Infof("start popping txs in block@%d", block.GetHeight())
				p.DelBlockTxs(block.Txs...)
			case vault := <-p.newVaultCh:
				log.Infof("new backend vault@%d for txpool", vault.GetHeight())
				p.CurrentVault = vault
			}
		}
	}()
}

func (p *TxPool) IsInPool(tx *ngtypes.Tx) (exists bool) {
	_, exists = p.Queuing[tx.GetConvener()]
	if !exists {
		return
	}

	exists = p.Queuing[tx.GetConvener()][tx.GetNonce()] != nil

	return
}
