package sheet

import (
	"bytes"
	"github.com/ngchain/ngcore/ngtypes"
)

// RegisterAccount is same to balanceSheet RegisterAccount, this is for consensus calling
func (sm *Manager) RegisterAccount(account *ngtypes.Account) (ok bool) {
	if _, exists := sm.accounts.Load(account.ID); !exists {
		sm.accounts.Store(account.ID, account)
		return true
	}

	return false
}

func (sm *Manager) DeleteAccount(account *ngtypes.Account) (ok bool) {
	if _, exists := sm.accounts.Load(account.ID); !exists {
		return false
	}

	sm.accounts.Delete(account.ID)
	return true
}

func (sm *Manager) AccountIsRegistered(accountID uint64) bool {
	_, exists := sm.accounts.Load(accountID)
	return exists
}

func (sm *Manager) GetAccountsByPublicKey(publicKey []byte) []*ngtypes.Account {
	accounts := make([]*ngtypes.Account, 0)
	sm.accounts.Range(func(_, account interface{}) bool {
		if bytes.Compare(account.(*ngtypes.Account).Owner, publicKey) == 0 {
			account = append(accounts, account.(*ngtypes.Account))
		}

		return true
	})

	return accounts
}
