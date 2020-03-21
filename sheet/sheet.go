package sheet

import (
	"github.com/ngchain/ngcore/ngtypes"
	"math/big"
)

func (sm *Manager) GenerateSheet() *ngtypes.Sheet {
	accounts := make(map[uint64]*ngtypes.Account)
	anonymous := make(map[string][]byte)

	sm.accounts.Range(func(height, account interface{}) bool {
		accounts[height.(uint64)] = account.(*ngtypes.Account)
		return true
	})

	sm.anonymous.Range(func(hexPK, balance interface{}) bool {
		anonymous[hexPK.(string)] = balance.(*big.Int).Bytes()
		return true
	})

	return ngtypes.NewSheet(accounts, anonymous)
}

// called when mined Vault
func (sm *Manager) GetSheetBytes() []byte {
	sheet := sm.GenerateSheet()
	b, err := sheet.Marshal()
	if err != nil {
		panic(err)
	}
	return b
}
