package ngsheet

import (
	"github.com/ngchain/ngcore/ngtypes"
)

// GenerateSheet will conclude a sheet which has all status of all accounts & keys(if balance not nil)
func (m *Manager) GenerateSheet() *ngtypes.Sheet {
	m.accountsMu.RLock()
	defer m.accountsMu.RUnlock()

	m.anonymousMu.RLock()
	defer m.anonymousMu.RUnlock()

	accounts := make(map[uint64]*ngtypes.Account)
	anonymous := make(map[string][]byte)

	for height, account := range m.accounts {
		accounts[height] = account
	}

	for hexPK, balance := range m.anonymous {
		anonymous[hexPK] = balance.Bytes()
	}

	return ngtypes.NewSheet(accounts, anonymous)
}

// GetSheetBytes is called when mined Vault
func (m *Manager) GetSheetBytes() []byte {
	sheet := m.GenerateSheet()
	b, err := sheet.Marshal()
	if err != nil {
		panic(err)
	}
	return b
}
