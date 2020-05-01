package ngsheet

import (
	"math/big"

	logging "github.com/ipfs/go-log/v2"

	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("sheet")

// StatusManager is a manager module
type StatusManager struct {
	// currentVault *ngtypes.Vault
	baseSheet    *state // the sheet from Vault, acts as the recovery
	currentSheet *state
}

var sheetManager *StatusManager

// GetSheetManager will create a Sheet manager
func GetSheetManager() *StatusManager {
	if sheetManager == nil {
		sheetManager = &StatusManager{
			baseSheet:    nil,
			currentSheet: nil,
		}
	}

	return sheetManager
}

// Init will initialize the Sheet manager with a specific vault and blocks on the vault
func (m *StatusManager) Init(latestBlocks *ngtypes.Block) {
	// TODO
}

// GetBalanceByNum is a helper to call GetBalanceByNum from currentSheet
func (m *StatusManager) GetBalanceByNum(id uint64) (*big.Int, error) {
	return m.currentSheet.GetBalanceByNum(id)
}

// CheckTxs is a helper to call CheckTxs from currentSheet
func (m *StatusManager) CheckTxs(txs ...*ngtypes.Tx) error {
	return m.currentSheet.CheckTxs()
}

func (m *StatusManager) HandleTxs(txs ...*ngtypes.Tx) error {
	log.Debugf("handling %d txs", len(txs))
	return m.currentSheet.handleTxs(txs...)
}

func (m *StatusManager) GenerateNewSheet() (*ngtypes.Sheet, error) {
	return m.currentSheet.ToSheet()
}

func (m *StatusManager) GetAccountsByPublicKey(key []byte) ([]*ngtypes.Account, error) {
	return m.currentSheet.getAccountsByPublicKey(key)
}

func (m *StatusManager) GetAccountByNum(id uint64) (*ngtypes.Account, error) {
	return m.currentSheet.getAccountByNum(id)
}

func (m *StatusManager) GetBalanceByPublicKey(pk []byte) (*big.Int, error) {
	return m.currentSheet.getBalanceByPublicKey(pk)
}

func (m *StatusManager) GetNextNonce(convener uint64) uint64 {
	return m.currentSheet.GetNextNonce(convener)
}
