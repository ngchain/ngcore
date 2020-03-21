package txpool

import (
	"errors"
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

	Queuing      map[uint64]map[uint64]*ngtypes.Transaction // key=txHash value=*TxEntry
	CurrentVault *ngtypes.Vault

	newBlockCh chan *ngtypes.Block
	newVaultCh chan *ngtypes.Vault

	NewCreatedTxEvent chan *ngtypes.Transaction

	expireCheckCh <-chan time.Time
}

func NewTxPool() *TxPool {
	return &TxPool{
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
	p.expireCheckCh = time.Tick(expireTTL)
	go func() {
		for {
			select {
			case <-p.expireCheckCh:
				p.DoExpireCheck()
				p.DetectEvil()
				break
			case block := <-p.newBlockCh:
				log.Infof("start popping txs in block@%d", block.GetHeight())
				p.DelTxs(block.Transactions...)
			case vault := <-p.newVaultCh:
				log.Infof("new backend vault@%d for txpool", vault.GetHeight())
				p.CurrentVault = vault
			}
		}
	}()
}

// TODO DetectEvil checks the tx is legal or not
func (p *TxPool) DetectEvil() {

}

// TODO: using this method in rpc
func (p *TxPool) PutNewTx(tx *ngtypes.Transaction) error {
	err := p.PutTxs(tx)
	if err != nil {
		return err
	}

	p.NewCreatedTxEvent <- tx

	return nil
}

func (p *TxPool) DelTxs(txs ...*ngtypes.Transaction) {
	p.Lock()
	defer p.Unlock()

	for i := range txs {
		if p.Queuing[txs[i].GetConvener()] != nil {
			delete(p.Queuing[txs[i].GetConvener()], txs[i].GetNonce())
		}
	}
}

func (p *TxPool) PutTxs(txs ...*ngtypes.Transaction) error {
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

func (p *TxPool) IsInPool(tx *ngtypes.Transaction) (exists bool) {
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
