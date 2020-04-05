package ngsheet

import (
	"github.com/ngchain/ngcore/ngtypes"
)

func (m *sheetEntry) GetNextNonce(accountID uint64) uint64 {
	m.RLock()
	defer m.RUnlock()

	rawAccount, exists := m.accounts[accountID]
	if !exists {
		return 1
	}

	account := new(ngtypes.Account)
	_ = account.Unmarshal(rawAccount)

	return account.GetNonce() + 1
}
