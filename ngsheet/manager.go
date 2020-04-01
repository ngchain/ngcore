package ngsheet

import (
	"encoding/hex"
	"math/big"
	"sync"

	"github.com/whyrusleeping/go-logging"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.MustGetLogger("sheet")

type Manager struct {
	currentVault *ngtypes.Vault

	accountsMu  sync.RWMutex
	anonymousMu sync.RWMutex

	accounts  map[uint64]*ngtypes.Account
	anonymous map[string]*big.Int
}

func NewSheetManager() *Manager {
	s := &Manager{
		currentVault: nil,
		accounts:     make(map[uint64]*ngtypes.Account),
		anonymous:    make(map[string]*big.Int),
	}

	return s
}

func (m *Manager) Init(currentVault *ngtypes.Vault) {
	log.Infof("sheet manager initialized on vault@%d", currentVault.Height)
	m.currentVault = currentVault
}

func (m *Manager) GetBalance(accountID uint64) (*big.Int, error) {
	m.accountsMu.RLock()
	account, exists := m.accounts[accountID]
	if !exists {
		return nil, ngtypes.ErrAccountNotExists
	}
	m.accountsMu.RUnlock()

	publicKey := hex.EncodeToString(account.Owner)

	m.anonymousMu.RLock()
	balance, exists := m.anonymous[publicKey]
	m.anonymousMu.RUnlock()
	if !exists {
		return nil, ngtypes.ErrAccountBalanceNotExists
	}

	return balance, nil
}
