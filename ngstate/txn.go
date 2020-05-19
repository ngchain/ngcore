package ngstate

import (
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// GetNextNonce gets the nonce for account's next tx
func (m *State) GetNextNonce(accountID uint64) uint64 {
	m.RLock()
	defer m.RUnlock()

	rawAccount, exists := m.accounts[accountID]
	if !exists {
		return 1
	}

	account := new(ngtypes.Account)
	_ = utils.Proto.Unmarshal(rawAccount, account)

	return account.GetTxn() + 1
}
