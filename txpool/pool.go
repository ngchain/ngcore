package txpool

import (
	"errors"
	"github.com/whyrusleeping/go-logging"
	"sync"
	"time"

	"github.com/ngin-network/ngcore/ngtypes"
)

const (
	expireTTL = time.Minute * 15
)

var log = logging.MustGetLogger("txpool")

// txPool is a little mem db which stores **signed** tx
type TxPool struct {
	sync.RWMutex
	Queuing      map[uint64]map[uint64]*ngtypes.Transaction // key=txHash value=*TxEntry
	CurrentVault *ngtypes.Vault

	Pack *ngtypes.TxTrie

	NewBlockCh    chan *ngtypes.Block
	NewVaultCh    chan *ngtypes.Vault
	expireCheckCh <-chan time.Time
}

func NewTxPool() *TxPool {
	return &TxPool{
		Queuing:      make(map[uint64]map[uint64]*ngtypes.Transaction),
		CurrentVault: nil,
	}
}

func (p *TxPool) Init(currentVault *ngtypes.Vault) {
	p.CurrentVault = currentVault
}

func (p *TxPool) Run() {
	p.expireCheckCh = time.Tick(expireTTL)
	go func() {
		select {
		case <-p.expireCheckCh:
			p.DoExpireCheck()
			p.DetectEvil()
			break
		case block := <-p.NewBlockCh:
			p.DelTxs(block.Transactions)
		case vault := <-p.NewVaultCh:
			p.CurrentVault = vault
		}
	}()
}

// TODO DetectEvil checks the tx is legal or not
func (p *TxPool) DetectEvil() {

}

func (p *TxPool) PutTx(tx *ngtypes.Transaction) error {
	return p.PutTxs([]*ngtypes.Transaction{tx})
}

func (p *TxPool) DelTxs(txs []*ngtypes.Transaction) {
	p.Lock()
	defer p.Unlock()

	for i := range txs {
		if p.Queuing[txs[i].GetConvener()] != nil {
			delete(p.Queuing[txs[i].GetConvener()], txs[i].GetNonce())
		}
	}
}

func (p *TxPool) PutTxs(txs []*ngtypes.Transaction) error {
	p.Lock()
	defer p.Unlock()

	var err error
	for i := range txs {
		if !txs[i].IsSigned() {
			err = errors.New("cannot putting unsigned tx, " + txs[i].HashHex() + " into queuing")
			log.Error(err)
			continue
		}

		if n := p.CurrentVault.Sheet.Accounts[txs[i].GetConvener()].Nonce + 1; txs[i].GetNonce() != n+1 {
			err = errors.New("Tx" + txs[i].HashHex() + "'s nonce is incorrect")
			log.Error(err)
			continue
		}

		p.Queuing[txs[i].GetConvener()][txs[i].GetNonce()] = txs[i]
	}

	return err
}

func (p *TxPool) InPool(tx *ngtypes.Transaction) (exists bool) {
	_, exists = p.Queuing[tx.GetConvener()]
	if exists == false {
		return
	}

	exists = p.Queuing[tx.GetConvener()][tx.GetNonce()] != nil

	return
}

// FIXME
func (p *TxPool) GetPack() *ngtypes.TxTrie {
	var slice []*ngtypes.Transaction
	for i := range p.Queuing {
		if p.Queuing[i] != nil && len(p.Queuing[i]) > 0 {
			for j := range p.Queuing[i] {
				slice = append(slice, p.Queuing[i][j])
			}
		}
	}
	trie := ngtypes.NewTxTrie(slice)
	trie.Sort()
	return trie
}

// FIXME
func (p *TxPool) GetPackTxs(maxSize int) []*ngtypes.Transaction {
	txs := p.GetPack().Txs
	size := 0
	//size += txs[0].Size()
	for i := 0; i < len(txs); i++ {
		size += txs[i].Size()
		if size > maxSize {
			return txs[:i]
		}
	}
	return txs
}

// TODO
func (p *TxPool) DoExpireCheck() {

}
