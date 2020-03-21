package sheet

import (
	"encoding/hex"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/whyrusleeping/go-logging"
	"math/big"
	"sync"
)

var log = logging.MustGetLogger("sheet")

type Manager struct {
	currentVault *ngtypes.Vault
	accounts     *sync.Map //map[uint64]*ngtypes.Account // TODO: sync.Map
	anonymous    *sync.Map //map[string]*big.Int  // TODO: sync.Map
}

func NewSheetManager() *Manager {
	s := &Manager{
		currentVault: nil,
		accounts:     &sync.Map{},
		anonymous:    &sync.Map{},
	}

	return s
}

func (sm *Manager) Init(currentVault *ngtypes.Vault) {
	sm.currentVault = currentVault
}

func (sm *Manager) GetBalance(accountID uint64) (*big.Int, error) {
	account, exists := sm.accounts.Load(accountID)
	if !exists {
		return nil, ngtypes.ErrAccountNotExists
	}

	pk := hex.EncodeToString(account.(*ngtypes.Account).Owner)
	balance, exists := sm.anonymous.Load(pk)
	if !exists {
		return nil, ngtypes.ErrAccountBalanceNotExists
	}
	return balance.(*big.Int), nil
}
