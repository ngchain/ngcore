package txpool

import (
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/whyrusleeping/go-logging"
	"sync"
	"time"

	"github.com/ngchain/ngcore/ngtypes"
)

const (
	expireTTL = time.Minute * 15
)

var log = logging.MustGetLogger("txpool")

// txPool is a little mem db which stores **signed** tx
type TxPool struct {
	sync.RWMutex

	sheetManager *ngsheet.Manager

	Queuing      map[uint64]map[uint64]*ngtypes.Transaction // map[accountID] map[nonce]Tx
	CurrentVault *ngtypes.Vault

	newBlockCh chan *ngtypes.Block
	newVaultCh chan *ngtypes.Vault

	NewCreatedTxEvent chan *ngtypes.Transaction

	expireCheckCh <-chan time.Time
}

func NewTxPool(sheetManager *ngsheet.Manager) *TxPool {
	return &TxPool{
		sheetManager: sheetManager,

		Queuing:      make(map[uint64]map[uint64]*ngtypes.Transaction),
		CurrentVault: nil,

		NewCreatedTxEvent: make(chan *ngtypes.Transaction),
	}
}

func (p *TxPool) Init(currentVault *ngtypes.Vault, newBlockCh chan *ngtypes.Block, newVaultCh chan *ngtypes.Vault) {
	p.CurrentVault = currentVault
	p.newBlockCh = newBlockCh
	p.newVaultCh = newVaultCh
}

func (p *TxPool) Run() {
	go func() {
		for {
			select {
			case block := <-p.newBlockCh:
				log.Infof("start popping txs in block@%d", block.GetHeight())
				p.DelBlockTxs(block.Transactions...)
			case vault := <-p.newVaultCh:
				log.Infof("new backend vault@%d for txpool", vault.GetHeight())
				p.CurrentVault = vault
				//p.CheckExpire() // no expire
			}
		}
	}()
}

func (p *TxPool) IsInPool(tx *ngtypes.Transaction) (exists bool) {
	_, exists = p.Queuing[tx.GetConvener()]
	if exists == false {
		return
	}

	exists = p.Queuing[tx.GetConvener()][tx.GetNonce()] != nil

	return
}
